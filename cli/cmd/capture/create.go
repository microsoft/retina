// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/internal/buildinfo"
	pkgcapture "github.com/microsoft/retina/pkg/capture"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
)

const (
	DefaultCleanUpAfterUpload bool          = false
	DefaultDebug              bool          = false
	DefaultDuration           time.Duration = 1 * time.Minute
	// DefaultHostPath is the default subpath (joined under DefaultHostPathBaseDir on
	// the node) where capture artifacts are stored. It is a bare name, not an
	// absolute path: absolute paths are rejected by the operator.
	DefaultHostPath string = "retina"
	// DefaultHostPathBaseDir is the default node-side parent directory for capture
	// artifacts; the user-supplied --host-path subpath is joined under it.
	DefaultHostPathBaseDir string        = "/var/log/retina/captures"
	DefaultIncludeMetadata bool          = true
	DefaultJobNumLimit     int           = 0
	DefaultMaxSize         int           = 100
	DefaultNodeSelectors   string        = "kubernetes.io/os=linux"
	DefaultNowait          bool          = true
	DefaultPacketSize      int           = 0
	DefaultS3Path          string        = "retina/captures"
	DefaultWaitPeriod      time.Duration = 1 * time.Minute
	DefaultWaitTimeout     time.Duration = 5 * time.Minute

	// JobTTLSecondsAfterFinished is how long completed/failed jobs and their
	// pods are kept before Kubernetes garbage-collects them (no-wait mode).
	JobTTLSecondsAfterFinished int32 = 300 // 5 minutes

	// JobActiveDeadlineBufferSeconds is added to the capture duration to form
	// the Job's activeDeadlineSeconds. This buffer accounts for output upload
	// and cleanup time after the capture itself finishes.
	JobActiveDeadlineBufferSeconds int64 = 1800 // 30 minutes

	// WaitDeadlineBuffer is extra time added to the capture duration when
	// computing the wait deadline, to account for upload and scheduling overhead.
	WaitDeadlineBuffer time.Duration = 5 * time.Minute

	// MinPollAttempts is the minimum number of polling iterations before the
	// wait deadline expires. Used to compute a fallback poll period.
	MinPollAttempts = 4
)

var errCleanupRequiresRemoteStorage = errors.New("--cleanup-after-upload requires remote storage (--blob-upload, --s3-bucket, or --pvc)")

// hasRemoteDestination returns true if the capture options specify a remote
// storage output (blob upload or S3 upload).
func hasRemoteDestination(o *Opts) bool {
	return o.blobUpload != "" || o.s3Bucket != "" || o.pvc != ""
}

var createExample = templates.Examples(i18n.T(`
		# Select nodes by node name and copy the artifacts to the node host path
		# (joined under the operator-configured base directory; default /var/log/retina/captures/testcapture)
		kubectl retina capture create --host-path testcapture --node-names "<nodename1>,<nodename2>"

		# Select pods determined by pod-selectors and namespace-selectors
		kubectl retina capture create --namespace capture --pod-selectors="k8s-app=kube-dns" --namespace-selectors="kubernetes.io/metadata.name=kube-system"

		# Select specific pods by name
		kubectl retina capture create --namespace default --pod-names "pod1,pod2" --duration 30s

		# Select a single pod by name
		kubectl retina capture create --namespace myapp --pod-names "my-app-pod-abc123" --duration 60s

		# Select nodes with label "agentpool=agentpool" and "version:v20"
		kubectl retina capture create --node-selectors="agentpool=agentpool,version:v20"

		# Select nodes using node-selector and set duration to 10s
		kubectl retina capture create --node-selectors="agentpool=agentpool" --duration=10s

		# Capture on specific network interfaces (instead of all interfaces)
		kubectl retina capture create --node-selectors="agentpool=agentpool" --interfaces="eth0,eth1"

		# Select nodes using node-selector and upload the artifacts to blob storage with SAS URL https://testaccount.blob.core.windows.net/<token>
		kubectl retina capture create --node-selectors="agentpool=agentpool" --blob-upload=https://testaccount.blob.core.windows.net/<token>

		# Select nodes using node-selector and upload the artifacts to AWS S3
		kubectl retina capture create --node-selectors="agentpool=agentpool" \
			--s3-bucket "your-bucket-name" \
			--s3-region "eu-central-1"\
			--s3-access-key-id "your-access-key-id" \
			--s3-secret-access-key "your-secret-access-key"

		# Select nodes using node-selector and upload the artifacts to S3-compatible service (like MinIO)
		kubectl retina capture create --node-selectors="agentpool=agentpool" \
			--s3-bucket "your-bucket-name" \
			--s3-endpoint "https://play.min.io:9000" \
			--s3-access-key-id "your-access-key-id" \
			--s3-secret-access-key "your-secret-access-key"
		`))

func create(kubeClient kubernetes.Interface) error {
	// Set namespace. If --namespace is not set, use namespace on user's context
	ns, _, err := opts.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return errors.Wrap(err, "failed to get namespace from kubeconfig")
	}

	if opts.Namespace == nil || *opts.Namespace == "" {
		opts.Namespace = &ns
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()

	capture, err := createCaptureF(ctx, kubeClient)
	if err != nil {
		return err
	}

	if opts.cleanUpAfterUpload {
		if !hasRemoteDestination(&opts) {
			return errCleanupRequiresRemoteStorage
		}
	}

	jobsCreated, err := createJobs(ctx, kubeClient, capture)
	if err != nil {
		opts.logger.Error("Failed to create job", zap.Error(err))
		return err
	}

	// Set owner references on secrets so they are garbage-collected when the
	// jobs are removed (by TTL, explicit deletion, or manual kubectl delete).
	if ownerRefErr := setSecretOwnerReferences(ctx, kubeClient, capture, jobsCreated); ownerRefErr != nil {
		opts.logger.Error("Failed to set owner references on capture secrets", zap.Error(ownerRefErr))
	}

	if opts.nowait {
		if opts.cleanUpAfterUpload && hasRemoteDestination(&opts) {
			opts.logger.Info("Capture jobs will be automatically cleaned up after upload (TTL-based)")
		} else {
			opts.logger.Info("Please manually delete all capture jobs (associated secrets will be garbage-collected automatically)")
		}
		printCaptureResult(jobsCreated)
		return nil
	}

	// Wait until all jobs finish then delete the jobs before the timeout, otherwise print jobs created to
	// let the customer recycle them.
	opts.logger.Info("Waiting for capture jobs to finish")

	allJobsCompleted := waitUntilJobsComplete(ctx, kubeClient, jobsCreated)

	// Delete all jobs created only if they all completed, otherwise keep the jobs for debugging.
	if allJobsCompleted {
		opts.logger.Info("Deleting jobs as all jobs are completed")
		jobsFailedToDelete := deleteJobs(ctx, kubeClient, jobsCreated)
		if len(jobsFailedToDelete) != 0 {
			opts.logger.Info("Please manually delete capture jobs failed to delete", zap.String("namespace", *opts.Namespace), zap.String("job list", strings.Join(jobsFailedToDelete, ",")))
		}

		if capture.Spec.OutputConfiguration.BlobUpload != nil {
			err = deleteSecret(ctx, kubeClient, capture.Spec.OutputConfiguration.BlobUpload)
			if err != nil {
				opts.logger.Error("Failed to delete capture secret, please manually delete it",
					zap.String("namespace", *opts.Namespace), zap.String("secret name", *capture.Spec.OutputConfiguration.BlobUpload), zap.Error(err))
			}
		}

		if capture.Spec.OutputConfiguration.S3Upload != nil && capture.Spec.OutputConfiguration.S3Upload.SecretName != "" {
			err = deleteSecret(ctx, kubeClient, &capture.Spec.OutputConfiguration.S3Upload.SecretName)
			if err != nil {
				opts.logger.Error("Failed to delete capture secret, please manually delete it",
					zap.String("namespace", *opts.Namespace),
					zap.String("secret name", capture.Spec.OutputConfiguration.S3Upload.SecretName),
					zap.Error(err),
				)
			}
		}

		if len(jobsFailedToDelete) == 0 && err == nil {
			opts.logger.Info("Done for deleting jobs")
		}
		return nil
	}

	opts.logger.Info("Not all job are completed in the given time")
	opts.logger.Info("Please manually delete the Capture")

	return getCaptureAndPrintCaptureResult(ctx, kubeClient, capture.Name, *opts.Namespace)
}

func GetClientset() (*kubernetes.Clientset, error) {
	kubeConfig, err := opts.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to compose k8s rest config")
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize kubernetes client")
	}

	return kubeClient, nil
}

func NewCreateSubCommand(kubeClient kubernetes.Interface) *cobra.Command {
	createCapture := &cobra.Command{
		Use:     "create",
		Short:   "Create a Retina Capture",
		Example: createExample,
	}

	var verbosityStr, timestampStr, printDataStr string

	createCapture.RunE = func(*cobra.Command, []string) error {
		// Validate enum flags
		opts.verbosityLevel = VerbosityLevel(verbosityStr)
		if err := opts.verbosityLevel.Validate(); err != nil {
			return err
		}
		opts.timestampFormat = TimestampFormat(timestampStr)
		if err := opts.timestampFormat.Validate(); err != nil {
			return err
		}
		opts.printDataFormat = PrintDataFormat(printDataStr)
		if err := opts.printDataFormat.Validate(); err != nil {
			return err
		}
		return create(kubeClient)
	}

	createCapture.Flags().DurationVar(&opts.duration, "duration", DefaultDuration, "Duration of capturing packets")
	createCapture.Flags().IntVar(&opts.maxSize, "max-size", DefaultMaxSize, "Limit the capture file to MB in size which works only for Linux") //nolint:gomnd // default
	createCapture.Flags().IntVar(&opts.packetSize, "packet-size", DefaultPacketSize, "Limits the each packet to bytes in size which works only for Linux")
	createCapture.Flags().StringVar(&opts.nodeNames, "node-names", "", "A comma-separated list of node names to select nodes on which the network capture will be performed")
	createCapture.Flags().StringVar(&opts.nodeSelectors, "node-selectors", DefaultNodeSelectors, "A comma-separated list of node labels to select nodes on which the network capture will be performed")
	createCapture.Flags().StringVar(&opts.podNames, "pod-names", "",
		"A comma-separated list of pod names to select specific pods on which the network capture will be performed (must be in the specified namespace)")
	createCapture.Flags().StringVar(&opts.podSelectors, "pod-selectors", "",
		"A comma-separated list of pod labels to select pods on which the network capture will be performed")
	createCapture.Flags().StringVar(&opts.namespaceSelectors, "namespace-selectors", "",
		"A comma-separated list of namespace labels to filter which namespaces will be targeted for packet capture (used with --pod-selectors)")
	createCapture.Flags().StringVar(&opts.hostPath, "host-path", DefaultHostPath,
		"Subpath name (joined under --host-path-base-dir) for capture artifacts on the node. Must be a relative subpath and must not contain '..'.")
	createCapture.Flags().StringVar(&opts.hostPathBaseDir, "host-path-base-dir", DefaultHostPathBaseDir, "Absolute base directory on the node under which --host-path is joined")
	createCapture.Flags().StringVar(&opts.pvc, "pvc", "", "PersistentVolumeClaim under the specified or default namespace to store capture files")
	createCapture.Flags().StringVar(&opts.blobUpload, "blob-upload", "", "Blob SAS URL with write permission to upload capture files")
	createCapture.Flags().StringVar(&opts.s3Region, "s3-region", "", "Region where the S3 compatible bucket is located")
	createCapture.Flags().StringVar(&opts.s3Endpoint, "s3-endpoint", "",
		"Endpoint for an S3 compatible storage service. Use this if you are using a custom or private S3 service that requires a specific endpoint")
	createCapture.Flags().StringVar(&opts.s3Bucket, "s3-bucket", "", "Bucket in which to store capture files")
	createCapture.Flags().StringVar(&opts.s3Path, "s3-path", DefaultS3Path, "Prefix path within the S3 bucket where captures will be stored")
	createCapture.Flags().StringVar(&opts.s3AccessKeyID, "s3-access-key-id", "", "S3 access key id to upload capture files")
	createCapture.Flags().StringVar(&opts.s3SecretAccessKey, "s3-secret-access-key", "", "S3 access secret key to upload capture files")
	createCapture.Flags().StringVar(&opts.tcpdumpFilter, "tcpdump-filter", "",
		"DEPRECATED and will be removed: Use --pcap-filter for BPF expressions. BPF filter expression without flags (e.g., 'host 10.0.0.1', 'tcp port 443')")
	createCapture.Flags().StringVar(&opts.pcapFilter, "pcap-filter", "",
		"BPF filter expression for packet filtering (e.g., 'host 10.0.0.1', 'tcp port 443'). See https://www.tcpdump.org/manpages/pcap-filter.7.html")
	createCapture.Flags().StringVar(&opts.interfaces, "interfaces", "", "Comma-separated list of network interfaces to capture on (e.g., eth0,eth1)")

	// Tcpdump boolean flags for capture behavior and display options
	createCapture.Flags().BoolVar(&opts.noPromiscuous, "no-promiscuous", false, "Disable promiscuous mode (tcpdump -p flag)")
	createCapture.Flags().BoolVar(&opts.packetBuffered, "packet-buffered", false, "Enable packet-buffered output (tcpdump -U flag)")
	createCapture.Flags().BoolVar(&opts.immediateMode, "immediate-mode", false, "Enable immediate mode for packet capture (tcpdump --immediate-mode)")
	createCapture.Flags().BoolVar(&opts.noResolveDNS, "no-resolve-dns", false, "Don't resolve hostnames (tcpdump -n flag)")
	createCapture.Flags().BoolVar(&opts.noResolvePort, "no-resolve-port", false, "Don't resolve hostnames or port names (tcpdump -nn flag)")
	createCapture.Flags().BoolVar(&opts.printLinkHeader, "print-link-header", false, "Print link-level headers (tcpdump -e flag)")
	createCapture.Flags().BoolVar(&opts.quietOutput, "quiet-output", false, "Quick/quiet output mode (tcpdump -q flag)")
	createCapture.Flags().BoolVar(&opts.absoluteSeq, "absolute-seq", false, "Print absolute TCP sequence numbers (tcpdump -S flag)")
	createCapture.Flags().BoolVar(&opts.dontVerifyChecksum, "dont-verify-checksum", false, "Don't verify TCP checksums (tcpdump -K flag)")

	// Enum-based flags for mutually exclusive options
	createCapture.Flags().StringVar(&verbosityStr, "verbosity", "", "Verbosity level: verbose, extra, max (tcpdump -v/-vv/-vvv)")
	createCapture.Flags().StringVar(&timestampStr, "timestamp-format", "", "Timestamp format: none, unformatted, delta, date, delta-since-first (tcpdump -t/-tt/-ttt/-tttt/-ttttt)")
	createCapture.Flags().StringVar(&printDataStr, "print-data", "", "Print packet data: hex, hex-with-link, ascii, ascii-with-link (tcpdump -x/-xx/-A/-AA)")

	// Filters
	createCapture.Flags().StringVar(&opts.excludeFilter, "exclude-filter", "", "A comma-separated list of IP:Port pairs that are "+
		"excluded from capturing network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*")
	createCapture.Flags().StringVar(&opts.includeFilter, "include-filter", "", "A comma-separated list of IP:Port pairs that are "+
		"used to filter capture network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*")

	// Capture options
	createCapture.Flags().BoolVar(&opts.includeMetadata, "include-metadata", DefaultIncludeMetadata, "If true, collect static network metadata into capture file")
	createCapture.Flags().IntVar(&opts.jobNumLimit, "job-num-limit", DefaultJobNumLimit, "The maximum number of jobs can be created for each capture. 0 means no limit")
	createCapture.Flags().BoolVar(&opts.nowait, "no-wait", DefaultNowait, "Do not wait for the long-running capture job to finish")
	createCapture.Flags().BoolVar(&opts.cleanUpAfterUpload, "cleanup-after-upload", DefaultCleanUpAfterUpload,
		"Automatically clean up capture jobs and resources after successful upload to remote storage")
	createCapture.Flags().BoolVar(&opts.debug, "debug", DefaultDebug, "When debug is true, a customized retina-agent image, determined by the environment variable RETINA_AGENT_IMAGE, is set")

	return createCapture
}

func createSecretFromBlobUpload(ctx context.Context, kubeClient kubernetes.Interface, blobUpload, captureName string) (string, error) {
	if blobUpload == "" {
		return "", nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: captureConstants.CaptureOutputLocationBlobUploadSecretName,
			Labels:       captureUtils.GetSerectLabelsFromCaptureName(captureName),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			captureConstants.CaptureOutputLocationBlobUploadSecretKey: []byte(blobUpload),
		},
	}
	secret, err := kubeClient.CoreV1().Secrets(*opts.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return secret.Name, nil
}

func createSecretFromS3Upload(ctx context.Context, kubeClient kubernetes.Interface, s3AccessKeyID, s3SecretAccessKey, captureName string) (string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: captureConstants.CaptureOutputLocationS3UploadSecretName,
			Labels:       captureUtils.GetSerectLabelsFromCaptureName(captureName),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			captureConstants.CaptureOutputLocationS3UploadAccessKeyID:     []byte(s3AccessKeyID),
			captureConstants.CaptureOutputLocationS3UploadSecretAccessKey: []byte(s3SecretAccessKey),
		},
	}
	secret, err := kubeClient.CoreV1().Secrets(*opts.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create s3 upload secret: %w", err)
	}
	return secret.Name, nil
}

func deleteSecret(ctx context.Context, kubeClient kubernetes.Interface, secretName *string) error {
	if secretName == nil {
		return nil
	}

	err := kubeClient.CoreV1().Secrets(*opts.Namespace).Delete(ctx, *secretName, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err //nolint:wrapcheck //internal return
}

// validateBPFFilter checks that a BPF filter string doesn't contain flags (tokens starting with '-').
// This prevents command injection attacks by ensuring only BPF expressions are provided.
func validateBPFFilter(filter, filterName string) error {
	if filter == "" {
		return nil
	}

	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return fmt.Errorf("%s: %w", filterName, ErrBPFFilterEmpty)
	}

	// Check for flags (tokens starting with '-')
	tokens := strings.Fields(trimmed)
	for _, token := range tokens {
		if strings.HasPrefix(token, "-") {
			return fmt.Errorf("%s contains flag %q: %w", filterName, token, ErrBPFFilterContainsFlag)
		}
	}

	return nil
}

func createCaptureF(ctx context.Context, kubeClient kubernetes.Interface) (*retinav1alpha1.Capture, error) {
	// Validate filters early to provide immediate feedback
	if err := validateBPFFilter(opts.tcpdumpFilter, "--tcpdump-filter"); err != nil {
		return nil, err
	}
	if err := validateBPFFilter(opts.pcapFilter, "--pcap-filter"); err != nil {
		return nil, err
	}

	timestamp := file.Now()

	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *opts.Name,
			Namespace: *opts.Namespace,
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				TcpdumpFilter:   &opts.tcpdumpFilter,
				CaptureTarget:   retinav1alpha1.CaptureTarget{},
				IncludeMetadata: opts.includeMetadata,
				CaptureOption:   retinav1alpha1.CaptureOption{},
			},
		},
		Status: retinav1alpha1.CaptureStatus{
			StartTime: timestamp,
		},
	}

	opts.logger.Info(fmt.Sprintf("Capture timestamp: %s", timestamp))

	if opts.duration != 0 {
		opts.logger.Info(fmt.Sprintf("The capture duration is set to %s", opts.duration))
		capture.Spec.CaptureConfiguration.CaptureOption.Duration = &metav1.Duration{Duration: opts.duration}
	}

	if opts.namespaceSelectors != "" || opts.podSelectors != "" || opts.podNames != "" || opts.nodeNames != "" {
		// if node selector is using the default value (aka hasn't been set by user), set it to nil to prevent clash
		// with namespace/pod selectors, pod names, or explicit node names
		if opts.nodeSelectors == DefaultNodeSelectors {
			opts.logger.Info("Overriding default node selectors value and setting it to nil. Using namespace, pod selectors, pod names, or node names. " +
				"To use node selector, please remove namespace and pod selectors.")
			opts.nodeSelectors = ""
		}
	}

	nodeSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(opts.nodeSelectors)
	if err != nil {
		return nil, err
	}
	podSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(opts.podSelectors)
	if err != nil {
		return nil, err
	}
	namespaceSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(opts.namespaceSelectors)
	if err != nil {
		return nil, err
	}

	if len(nodeSelectorLabelsMap) != 0 || opts.nodeNames != "" {
		capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector = &metav1.LabelSelector{}
	}
	if len(nodeSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchLabels = nodeSelectorLabelsMap
	}
	if opts.nodeNames != "" {
		nodeNameSlice := strings.Split(opts.nodeNames, ",")
		if len(nodeNameSlice) != 0 {
			capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
				Key:      corev1.LabelHostname,
				Operator: metav1.LabelSelectorOpIn,
				Values:   nodeNameSlice,
			}}
		}
	}

	// Add namespace selectors if provided, regardless of other selectors
	if len(namespaceSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: namespaceSelectorLabelsMap,
		}
	}

	// Add pod selectors if provided
	if len(podSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.PodSelector = &metav1.LabelSelector{
			MatchLabels: podSelectorLabelsMap,
		}
	}

	// Add pod names if provided
	if opts.podNames != "" {
		podNameSlice := strings.Split(opts.podNames, ",")
		for i := range podNameSlice {
			podNameSlice[i] = strings.TrimSpace(podNameSlice[i])
		}
		opts.logger.Info(fmt.Sprintf("Capturing on specific pods: %v", podNameSlice))
		capture.Spec.CaptureConfiguration.CaptureTarget.PodNames = podNameSlice
	}

	if opts.maxSize != 0 {
		opts.logger.Info(fmt.Sprintf("The capture file max size is set to %dMB", opts.maxSize))
		capture.Spec.CaptureConfiguration.CaptureOption.MaxCaptureSize = &opts.maxSize
	}

	if opts.packetSize != 0 {
		opts.logger.Info(fmt.Sprintf("The capture packet size is set to %d bytes", opts.packetSize))
		capture.Spec.CaptureConfiguration.CaptureOption.PacketSize = &opts.packetSize
	}

	if opts.interfaces != "" {
		interfaceSlice := strings.Split(opts.interfaces, ",")
		for i := range interfaceSlice {
			interfaceSlice[i] = strings.TrimSpace(interfaceSlice[i])
		}
		opts.logger.Info(fmt.Sprintf("Capturing on specific interfaces: %v", interfaceSlice))
		capture.Spec.CaptureConfiguration.CaptureOption.Interfaces = interfaceSlice
	}

	// Set pcap-filter if provided
	if opts.pcapFilter != "" {
		capture.Spec.CaptureConfiguration.CaptureOption.PcapFilter = &opts.pcapFilter
	}

	// Set boolean capture and display flags
	if opts.noPromiscuous {
		capture.Spec.CaptureConfiguration.CaptureOption.NoPromiscuous = &opts.noPromiscuous
	}

	if opts.packetBuffered {
		capture.Spec.CaptureConfiguration.CaptureOption.PacketBuffered = &opts.packetBuffered
	}

	if opts.immediateMode {
		capture.Spec.CaptureConfiguration.CaptureOption.ImmediateMode = &opts.immediateMode
	}

	if opts.noResolveDNS {
		capture.Spec.CaptureConfiguration.CaptureOption.NoResolveDNS = &opts.noResolveDNS
	}
	if opts.noResolvePort {
		capture.Spec.CaptureConfiguration.CaptureOption.NoResolvePort = &opts.noResolvePort
	}

	// Set verbosity level enum field based on CLI value
	if opts.verbosityLevel != VerbosityNormal {
		verbosityStr := string(opts.verbosityLevel)
		capture.Spec.CaptureConfiguration.CaptureOption.Verbosity = &verbosityStr
	}

	// Set print data format enum field based on CLI value
	if opts.printDataFormat != PrintDataNone {
		printDataStr := string(opts.printDataFormat)
		capture.Spec.CaptureConfiguration.CaptureOption.PrintDataFormat = &printDataStr
	}

	if opts.printLinkHeader {
		capture.Spec.CaptureConfiguration.CaptureOption.PrintLinkHeader = &opts.printLinkHeader
	}

	if opts.quietOutput {
		capture.Spec.CaptureConfiguration.CaptureOption.QuietOutput = &opts.quietOutput
	}

	if opts.absoluteSeq {
		capture.Spec.CaptureConfiguration.CaptureOption.AbsoluteSeq = &opts.absoluteSeq
	}

	// Set timestamp format enum field based on CLI value
	if opts.timestampFormat != TimestampDefault {
		timestampStr := string(opts.timestampFormat)
		capture.Spec.CaptureConfiguration.CaptureOption.TimestampFormat = &timestampStr
	}

	if opts.dontVerifyChecksum {
		capture.Spec.CaptureConfiguration.CaptureOption.DontVerifyChecksum = &opts.dontVerifyChecksum
	}

	if opts.hostPath != "" {
		capture.Spec.OutputConfiguration.HostPath = &opts.hostPath
	}
	if opts.pvc != "" {
		capture.Spec.OutputConfiguration.PersistentVolumeClaim = &opts.pvc
	}

	if opts.blobUpload != "" {
		// Mount blob url as secret onto the capture pod for security concern if blob url is not empty.
		secretName, err := createSecretFromBlobUpload(ctx, kubeClient, opts.blobUpload, *opts.Name)
		if err != nil {
			return nil, err
		}
		capture.Spec.OutputConfiguration.BlobUpload = &secretName
	}

	if opts.s3Bucket != "" {
		secretName, err := createSecretFromS3Upload(ctx, kubeClient, opts.s3AccessKeyID, opts.s3SecretAccessKey, *opts.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create s3 upload secret: %w", err)
		}
		capture.Spec.OutputConfiguration.S3Upload = &retinav1alpha1.S3Upload{
			Endpoint:   opts.s3Endpoint,
			Bucket:     opts.s3Bucket,
			SecretName: secretName,
			Region:     opts.s3Region,
			Path:       opts.s3Path,
		}
	}

	if opts.excludeFilter != "" {
		if capture.Spec.CaptureConfiguration.Filters == nil {
			capture.Spec.CaptureConfiguration.Filters = &retinav1alpha1.CaptureConfigurationFilters{}
		}
		excludeFilterSlice := strings.Split(opts.excludeFilter, ",")
		capture.Spec.CaptureConfiguration.Filters.Exclude = excludeFilterSlice
	}

	if opts.includeFilter != "" {
		if capture.Spec.CaptureConfiguration.Filters == nil {
			capture.Spec.CaptureConfiguration.Filters = &retinav1alpha1.CaptureConfigurationFilters{}
		}
		includeFilterSlice := strings.Split(opts.includeFilter, ",")
		capture.Spec.CaptureConfiguration.Filters.Include = includeFilterSlice
	}

	capture.Spec.CleanUpAfterUpload = opts.cleanUpAfterUpload
	return capture, nil
}

func getCLICaptureConfig() config.CaptureConfig {
	return config.CaptureConfig{
		CaptureImageVersion:       buildinfo.Version,
		CaptureDebug:              opts.debug,
		CaptureImageVersionSource: captureUtils.VersionSourceCLIVersion,
		CaptureJobNumLimit:        opts.jobNumLimit,
		CaptureHostPathBaseDir:    opts.hostPathBaseDir,
	}
}

func createJobs(ctx context.Context, kubeClient kubernetes.Interface, capture *retinav1alpha1.Capture) ([]batchv1.Job, error) {
	translator := pkgcapture.NewCaptureToPodTranslator(kubeClient, opts.logger, getCLICaptureConfig())
	jobs, err := translator.TranslateCaptureToJobs(ctx, capture)
	if err != nil {
		return nil, err
	}

	jobsCreated := []batchv1.Job{}
	for _, job := range jobs {
		// In no-wait mode with a remote destination, set TTL so Kubernetes
		// auto-deletes completed jobs and their pods. When host-path only,
		// the job metadata is needed to locate the capture file.
		if opts.nowait && opts.cleanUpAfterUpload && hasRemoteDestination(&opts) {
			ttl := JobTTLSecondsAfterFinished
			job.Spec.TTLSecondsAfterFinished = &ttl
		}

		// Set a hard deadline to prevent jobs from running indefinitely if the
		// capture process hangs. Deadline = capture duration + upload buffer.
		if opts.duration > 0 {
			deadline := int64(opts.duration.Seconds()) + JobActiveDeadlineBufferSeconds
			job.Spec.ActiveDeadlineSeconds = &deadline
		}

		jobCreated, err := kubeClient.BatchV1().Jobs(*opts.Namespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		jobsCreated = append(jobsCreated, *jobCreated)
		opts.logger.Info("Packet capture job is created", zap.String("namespace", *opts.Namespace), zap.String("capture job", jobCreated.Name))
	}
	return jobsCreated, nil
}

func waitUntilJobsComplete(ctx context.Context, kubeClient kubernetes.Interface, jobs []batchv1.Job) bool {
	allJobsCompleted := false

	// TODO: let's make the timeout and period to wait for all job to finish configurable.
	deadline := DefaultWaitTimeout
	if opts.duration != 0 {
		// Allow time for capture + upload + scheduling overhead (Windows
		// nodes in particular need extra time for image pull and pod start).
		deadline = opts.duration + WaitDeadlineBuffer
		if deadline < DefaultWaitTimeout {
			deadline = DefaultWaitTimeout
		}
	}

	period := DefaultWaitPeriod
	// To print less noisy messages, we rely on duration to decide the wait period.
	if period < opts.duration/10 {
		period = opts.duration / 10
	}
	// Ensure poll period is less than the deadline so we get multiple checks.
	if period >= deadline {
		period = deadline / MinPollAttempts
	}
	opts.logger.Info(fmt.Sprintf("Waiting timeout is set to %s", deadline))

	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	wait.JitterUntil(func() {
		jobsCompleted := []string{}
		jobsIncompleted := []string{}

		for _, job := range jobs {
			jobRet, err := kubeClient.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
			if err != nil {
				opts.logger.Error("Failed to get job", zap.String("namespace", job.Namespace), zap.String("job name", job.Name), zap.Error(err))
				jobsIncompleted = append(jobsIncompleted, job.Name)
				continue
			}
			if jobRet.Status.CompletionTime != nil {
				jobsCompleted = append(jobsCompleted, job.Name)
			} else {
				jobsIncompleted = append(jobsIncompleted, job.Name)
			}
		}

		if len(jobsIncompleted) != 0 {
			opts.logger.Info("Not all jobs are completed",
				zap.String("namespace", *opts.Namespace),
				zap.String("Completed jobs", strings.Join(jobsCompleted, ",")),
				zap.String("Uncompleted packet capture jobs", strings.Join(jobsIncompleted, ",")),
			)
			// Return to have another try after an interval.
			return
		}
		allJobsCompleted = true
		cancel()
	}, period, 0.2, true, ctx.Done())

	return allJobsCompleted
}

func deleteJobs(ctx context.Context, kubeClient kubernetes.Interface, jobs []batchv1.Job) []string {
	jobsFailedtoDelete := []string{}
	for _, job := range jobs {
		// Child pods are preserved by default when jobs are deleted, we need to set propagationPolicy=Background to
		// remove them.
		deletePropagationBackground := metav1.DeletePropagationBackground
		err := kubeClient.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationBackground,
		})
		if err != nil {
			jobsFailedtoDelete = append(jobsFailedtoDelete, job.Name)
			opts.logger.Error("Failed to delete job", zap.String("namespace", job.Namespace), zap.String("job name", job.Name), zap.Error(err))
		}
	}
	return jobsFailedtoDelete
}

// setSecretOwnerReferences adds ownerReferences from the given jobs to the
// secrets used by the capture (blob or S3). This ensures secrets are garbage-
// collected when the owning jobs are deleted (by TTL or explicitly).
func setSecretOwnerReferences(ctx context.Context, kubeClient kubernetes.Interface, capture *retinav1alpha1.Capture, jobs []batchv1.Job) error {
	var secretNames []string
	if capture.Spec.OutputConfiguration.BlobUpload != nil {
		secretNames = append(secretNames, *capture.Spec.OutputConfiguration.BlobUpload)
	}
	if capture.Spec.OutputConfiguration.S3Upload != nil && capture.Spec.OutputConfiguration.S3Upload.SecretName != "" {
		secretNames = append(secretNames, capture.Spec.OutputConfiguration.S3Upload.SecretName)
	}

	if len(secretNames) == 0 || len(jobs) == 0 {
		return nil
	}

	ownerRefs := make([]metav1.OwnerReference, 0, len(jobs))
	for i := range jobs {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			APIVersion: batchv1.SchemeGroupVersion.String(),
			Kind:       "Job",
			Name:       jobs[i].Name,
			UID:        jobs[i].UID,
		})
	}

	for _, secretName := range secretNames {
		secret, err := kubeClient.CoreV1().Secrets(*opts.Namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get secret %s: %w", secretName, err)
		}
		// Deduplicate owner references to avoid appending the same refs on retries.
		existing := make(map[types.UID]struct{}, len(secret.OwnerReferences))
		for _, ref := range secret.OwnerReferences {
			existing[ref.UID] = struct{}{}
		}
		for _, ref := range ownerRefs {
			if _, found := existing[ref.UID]; !found {
				secret.OwnerReferences = append(secret.OwnerReferences, ref)
			}
		}
		if _, err := kubeClient.CoreV1().Secrets(*opts.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update secret %s with owner references: %w", secretName, err)
		}
	}
	return nil
}
