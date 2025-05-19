package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/shell"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	configFlags              *genericclioptions.ConfigFlags
	matchVersionFlags        *cmdutil.MatchVersionFlags
	retinaShellImageRepo     string
	retinaShellImageVersion  string
	windowsImageTag          string
	mountHostFilesystem      bool
	allowHostFilesystemWrite bool
	hostPID                  bool
	capabilities             []string
	timeout                  time.Duration
)

var (
	// AKS requires clusters to allow access to MCR, so use this repository by default.
	defaultRetinaShellImageRepo = "mcr.microsoft.com/containernetworking/retina-shell"

	// Default version is the same as CLI version, set at link time.
	defaultRetinaShellImageVersion = buildinfo.Version

	defaultTimeout = 30 * time.Second

	// Default Windows image tag suffix
	defaultWindowsImageTag = "windows-ltsc2022-amd64"

	errMissingRequiredRetinaShellImageVersionArg = errors.New("missing required --retina-shell-image-version")
	errUnsupportedResourceType                   = errors.New("unsupported resource type")
)

var shellCmd = &cobra.Command{
	Use:   "shell (NODE | TYPE[[.VERSION].GROUP]/NAME)",
	Short: "[EXPERIMENTAL] Interactively debug a node or pod",
	Long: templates.LongDesc(`
	[EXPERIMENTAL] This is an experimental command. The flags and behavior may change in the future.

	Start a shell with networking tools in a node or pod for adhoc debugging.

	* For nodes, this creates a pod on the node in the root network namespace.
	* For pods, this creates an ephemeral container inside the pod's network namespace.

	You can override the default image used for the shell container with either
	CLI flags (--retina-shell-image-repo and --retina-shell-image-version) or
	environment variables (RETINA_SHELL_IMAGE_REPO and RETINA_SHELL_IMAGE_VERSION).
	CLI flags take precedence over env vars.

	For Windows nodes, the shell image will automatically use the Windows variant with the
	specified Windows image tag suffix (--windows-image-tag). Windows support requires a 
	Windows node with HostProcess containers enabled.
`),

	Example: templates.Examples(`
		# start a shell in a node
		kubectl retina shell node0001

		# start a shell in a node, with debug pod in kube-system namespace
		kubectl retina shell -n kube-system node0001

		# start a shell as an ephemeral container inside an existing pod
		kubectl retina shell -n kube-system pod/coredns-d459997b4-7cpzx

		# start a shell in a node, mounting the host filesystem to /host with ability to chroot
		kubectl retina shell node001 --mount-host-filesystem --capabilities SYS_CHROOT

		# start a shell in a node, with NET_RAW and NET_ADMIN capabilities
		# (required for iptables and tcpdump)
		kubectl retina shell node001 --capabilities NET_RAW,NET_ADMIN

		# start a shell in a Windows node
		kubectl retina shell win-node001
`),
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		// retinaShellImageVersion defaults to the CLI version, but that might not be set if the CLI is built without -ldflags.
		if retinaShellImageVersion == "" {
			return errMissingRequiredRetinaShellImageVersionArg
		}

		namespace, explicitNamespace, err := matchVersionFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("error retrieving namespace arg: %w", err)
		}

		// This interprets the first arg as either a node or pod (same as kubectl):
		//   "node001"           -> node
		//   "node/node001"      -> node
		//   "pod/example-7cpzx" -> pod
		r := resource.NewBuilder(configFlags).
			WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
			FilenameParam(explicitNamespace, &resource.FilenameOptions{}).
			NamespaceParam(namespace).DefaultNamespace().ResourceNames("nodes", args[0]).
			Do()
		if rerr := r.Err(); rerr != nil {
			return fmt.Errorf("error constructing resource builder: %w", rerr)
		}

		restConfig, err := matchVersionFlags.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("error constructing REST config: %w", err)
		}

		// Create Kubernetes clientset to determine node OS
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("error creating clientset: %w", err)
		}

		// Create a generic config for now, we'll update image based on node OS
		config := shell.Config{
			RestConfig:               restConfig,
			RetinaShellImage:         fmt.Sprintf("%s:%s", retinaShellImageRepo, retinaShellImageVersion),
			MountHostFilesystem:      mountHostFilesystem,
			AllowHostFilesystemWrite: allowHostFilesystemWrite,
			HostPID:                  hostPID,
			Capabilities:             capabilities,
			Timeout:                  timeout,
		}

		return r.Visit(func(info *resource.Info, err error) error {
			if err != nil {
				return err
			}

			switch obj := info.Object.(type) {
			case *v1.Node:
				nodeName := obj.Name
				podDebugNamespace := namespace

				// Get the OS and update the image if it's Windows
				nodeOS := obj.Labels["kubernetes.io/os"]
				if nodeOS == "windows" {
					// For Windows, use the Windows-specific image tag
					windowsImageVersion := fmt.Sprintf("%s-%s", retinaShellImageVersion, windowsImageTag)
					config.RetinaShellImage = fmt.Sprintf("%s:%s", retinaShellImageRepo, windowsImageVersion)
					fmt.Printf("Using Windows shell image: %s\n", config.RetinaShellImage)
				}

				return shell.RunInNode(config, nodeName, podDebugNamespace)
			case *v1.Pod:
				// For pods, we need to get the node OS based on the pod's node
				ctx := context.Background()
				nodeOS, err := shell.GetNodeOS(ctx, clientset, obj.Spec.NodeName)
				if err != nil {
					return fmt.Errorf("error getting node OS: %w", err)
				}

				if nodeOS == "windows" {
					// For Windows, use the Windows-specific image tag
					windowsImageVersion := fmt.Sprintf("%s-%s", retinaShellImageVersion, windowsImageTag)
					config.RetinaShellImage = fmt.Sprintf("%s:%s", retinaShellImageRepo, windowsImageVersion)
					fmt.Printf("Using Windows shell image: %s\n", config.RetinaShellImage)
				}

				return shell.RunInPod(config, obj.Namespace, obj.Name)
			default:
				gvk := obj.GetObjectKind().GroupVersionKind()
				return fmt.Errorf("unsupported resource %s/%s: %w", gvk.GroupVersion(), gvk.Kind, errUnsupportedResourceType)
			}
		})
	},
}

func init() {
	Retina.AddCommand(shellCmd)
	shellCmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		// Avoid printing full usage message if the command exits with an error.
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		// Allow setting image repo and version via environment variables (CLI flags still take precedence).
		if !cmd.Flags().Changed("retina-shell-image-repo") {
			if envRepo := os.Getenv("RETINA_SHELL_IMAGE_REPO"); envRepo != "" {
				retinaShellImageRepo = envRepo
			}
		}
		if !cmd.Flags().Changed("retina-shell-image-version") {
			if envVersion := os.Getenv("RETINA_SHELL_IMAGE_VERSION"); envVersion != "" {
				retinaShellImageVersion = envVersion
			}
		}
		if !cmd.Flags().Changed("windows-image-tag") {
			if envWindowsTag := os.Getenv("RETINA_SHELL_WINDOWS_IMAGE_TAG"); envWindowsTag != "" {
				windowsImageTag = envWindowsTag
			}
		}
	}
	shellCmd.Flags().StringVar(&retinaShellImageRepo, "retina-shell-image-repo", defaultRetinaShellImageRepo, "The container registry repository for the image to use for the shell container")
	shellCmd.Flags().StringVar(&retinaShellImageVersion, "retina-shell-image-version", defaultRetinaShellImageVersion, "The version (tag) of the image to use for the shell container")
	shellCmd.Flags().StringVar(&windowsImageTag, "windows-image-tag", defaultWindowsImageTag, "The tag suffix to use for Windows shell images (e.g., 'windows-ltsc2022-amd64')")
	shellCmd.Flags().BoolVarP(&mountHostFilesystem, "mount-host-filesystem", "m", false, "Mount the host filesystem to /host. Applies only to nodes, not pods.")
	shellCmd.Flags().BoolVarP(&allowHostFilesystemWrite, "allow-host-filesystem-write", "w", false,
		"Allow write access to the host filesystem. Implies --mount-host-filesystem. Applies only to nodes, not pods.")
	shellCmd.Flags().BoolVar(&hostPID, "host-pid", false, "Set HostPID on the shell container. Applies only to nodes, not pods.")
	shellCmd.Flags().StringSliceVarP(&capabilities, "capabilities", "c", []string{}, "Add capabilities to the shell container")
	shellCmd.Flags().DurationVar(&timeout, "timeout", defaultTimeout, "The maximum time to wait for the shell container to start")

	// configFlags and matchVersion flags are used to load kubeconfig.
	// This uses the same mechanism as `kubectl debug` to connect to apiserver and attach to containers.
	configFlags = genericclioptions.NewConfigFlags(true)
	configFlags.AddFlags(shellCmd.PersistentFlags())
	matchVersionFlags = cmdutil.NewMatchVersionFlags(configFlags)
	matchVersionFlags.AddFlags(shellCmd.PersistentFlags())
}
