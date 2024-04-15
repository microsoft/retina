// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"strings"
	"time"

	retinacmd "github.com/microsoft/retina/cli/cmd"
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	pkgcapture "github.com/microsoft/retina/pkg/capture"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	configFlags *genericclioptions.ConfigFlags

	duration           time.Duration
	maxSize            int
	packetSize         int
	nodeSelectors      string
	podSelectors       string
	namespaceSelectors string
	nodeNames          string
	hostPath           string
	pvc                string
	blobUpload         string
	tcpdumpFilter      string
	excludeFilter      string
	includeFilter      string
	includeMetadata    bool
	namespace          string
	jobNumLimit        int

	nowait bool

	debug bool
)

var createExample = templates.Examples(i18n.T(`
		# Capture network packets on the node selected by node names and copy the artifacts to the node host path /mnt/capture
		kubectl retina capture create --host-path /mnt/capture --namespace capture --node-names "aks-nodepool1-41844487-vmss000000,aks-nodepool1-41844487-vmss000001"

		# Capture network packets on the coredns pods determined by pod-selectors and namespace-selectors
		kubectl retina capture create --host-path /mnt/capture --namespace capture --pod-selectors="k8s-app=kube-dns" --namespace-selectors="kubernetes.io/metadata.name=kube-system"

		# Capture network packets on nodes with label "agentpool=agentpool" and "version:v20"
		kubectl retina capture create --host-path /mnt/capture --node-selectors="agentpool=agentpool,version:v20"

		# Capture network packets on nodes using node-selector with duration 10s
		kubectl retina capture create --host-path=/mnt/capture --node-selectors="agentpool=agentpool" --duration=10s

		# Capture network packets on nodes using node-selector and upload the artifacts to blob storage with SAS URL https://testaccount.blob.core.windows.net/<token>
		kubectl retina capture create --node-selectors="agentpool=agentpool" --blob-upload=https://testaccount.blob.core.windows.net/<token>
		`))

const (
	defaultWaitTimeout time.Duration = 5 * time.Minute
	defaultWaitPeriod  time.Duration = 1 * time.Minute
)

var createCapture = &cobra.Command{
	Use:     "create",
	Short:   "create a Retina Capture",
	Example: createExample,
	RunE: func(*cobra.Command, []string) error {
		kubeConfig, err := configFlags.ToRESTConfig()
		if err != nil {
			return errors.Wrap(err, "failed to compose k8s rest config")
		}

		kubeClient, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return errors.Wrap(err, "failed to initialize kubernetes client")
		}

		capture, err := createCaptureF(kubeClient)
		if err != nil {
			return err
		}

		jobsCreated, err := createJobs(kubeClient, capture)
		if err != nil {
			retinacmd.Logger.Error("Failed to create job", zap.Error(err))
			return err
		}
		if nowait {
			retinacmd.Logger.Info("Please manually delete all capture jobs")
			if capture.Spec.OutputConfiguration.BlobUpload != nil {
				retinacmd.Logger.Info("Please manually delete capture secret", zap.String("namespace", namespace), zap.String("secret name", *capture.Spec.OutputConfiguration.BlobUpload))
			}
			printCaptureResult(jobsCreated)
			return nil
		}

		// Wait until all jobs finish then delete the jobs before the timeout, otherwise print jobs created to
		// let the customer recycle them.
		retinacmd.Logger.Info("Waiting for capture jobs to finish")

		allJobsCompleted := waitUntilJobsComplete(kubeClient, jobsCreated)

		// Delete all jobs created only if they all completed, otherwise keep the jobs for debugging.
		if allJobsCompleted {
			retinacmd.Logger.Info("Deleting jobs as all jobs are completed")
			jobsFailedToDelete := deleteJobs(kubeClient, jobsCreated)
			if len(jobsFailedToDelete) != 0 {
				retinacmd.Logger.Info("Please manually delete capture jobs failed to delete", zap.String("namespace", namespace), zap.String("job list", strings.Join(jobsFailedToDelete, ",")))
			}

			err = deleteSecret(kubeClient, capture.Spec.OutputConfiguration.BlobUpload)
			if err != nil {
				retinacmd.Logger.Error("Failed to delete capture secret, please manually delete it",
					zap.String("namespace", namespace), zap.String("secret name", *capture.Spec.OutputConfiguration.BlobUpload), zap.Error(err))
			}

			if len(jobsFailedToDelete) == 0 && err == nil {
				retinacmd.Logger.Info("Done for deleting jobs")
			}
			return nil
		}

		retinacmd.Logger.Info("Not all job are completed in the given time")
		retinacmd.Logger.Info("Please manually delete the Capture")
		return getCaptureAndPrintCaptureResult(kubeClient, capture.Name, namespace)
	},
}

func createSecretFromBlobUpload(kubeClient kubernetes.Interface, blobUpload, captureName string) (string, error) {
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
	secret, err := kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return secret.Name, nil
}

func deleteSecret(kubeClient kubernetes.Interface, secretName *string) error {
	if secretName == nil {
		return nil
	}

	return kubeClient.CoreV1().Secrets(namespace).Delete(context.TODO(), *secretName, metav1.DeleteOptions{})
}

func createCaptureF(kubeClient kubernetes.Interface) (*retinav1alpha1.Capture, error) {
	captureName := fmt.Sprintf("retina-capture-%s", utilrand.String(5))
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:      captureName,
			Namespace: namespace,
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				TcpdumpFilter:   &tcpdumpFilter,
				CaptureTarget:   retinav1alpha1.CaptureTarget{},
				IncludeMetadata: includeMetadata,
				CaptureOption:   retinav1alpha1.CaptureOption{},
			},
		},
	}

	if duration != 0 {
		retinacmd.Logger.Info(fmt.Sprintf("The capture duration is set to %s", duration))
		capture.Spec.CaptureConfiguration.CaptureOption.Duration = &metav1.Duration{Duration: duration}
	}

	nodeSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(nodeSelectors)
	if err != nil {
		return nil, err
	}
	podSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(podSelectors)
	if err != nil {
		return nil, err
	}
	namespaceSelectorLabelsMap, err := labels.ConvertSelectorToLabelsMap(namespaceSelectors)
	if err != nil {
		return nil, err
	}

	if len(nodeSelectorLabelsMap) != 0 || len(nodeNames) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector = &metav1.LabelSelector{}
	}
	if len(nodeSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchLabels = nodeSelectorLabelsMap
	}
	if len(nodeNames) != 0 {
		nodeNameSlice := strings.Split(nodeNames, ",")
		if len(nodeNameSlice) != 0 {
			capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
				Key:      corev1.LabelHostname,
				Operator: metav1.LabelSelectorOpIn,
				Values:   nodeNameSlice,
			}}
		}
	}

	if len(namespaceSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: namespaceSelectorLabelsMap,
		}
	}
	if len(podSelectorLabelsMap) != 0 {
		capture.Spec.CaptureConfiguration.CaptureTarget.PodSelector = &metav1.LabelSelector{
			MatchLabels: podSelectorLabelsMap,
		}
	}

	if maxSize != 0 {
		retinacmd.Logger.Info(fmt.Sprintf("The capture file max size is set to %dMB", maxSize))
		capture.Spec.CaptureConfiguration.CaptureOption.MaxCaptureSize = &maxSize
	}

	if packetSize != 0 {
		retinacmd.Logger.Info(fmt.Sprintf("The capture packet size is set to %d bytes", packetSize))
		capture.Spec.CaptureConfiguration.CaptureOption.PacketSize = &packetSize
	}

	if len(hostPath) != 0 {
		capture.Spec.OutputConfiguration.HostPath = &hostPath
	}
	if len(pvc) != 0 {
		capture.Spec.OutputConfiguration.PersistentVolumeClaim = &pvc
	}

	if len(blobUpload) != 0 {
		// Mount blob url as secret onto the capture pod for security concern if blob url is not empty.
		secretName, err := createSecretFromBlobUpload(kubeClient, blobUpload, captureName)
		if err != nil {
			return nil, err
		}
		capture.Spec.OutputConfiguration.BlobUpload = &secretName
	}

	if len(excludeFilter) != 0 {
		if capture.Spec.CaptureConfiguration.Filters == nil {
			capture.Spec.CaptureConfiguration.Filters = &retinav1alpha1.CaptureConfigurationFilters{}
		}
		excludeFilterSlice := strings.Split(excludeFilter, ",")
		capture.Spec.CaptureConfiguration.Filters.Exclude = excludeFilterSlice
	}

	if len(includeFilter) != 0 {
		if capture.Spec.CaptureConfiguration.Filters == nil {
			capture.Spec.CaptureConfiguration.Filters = &retinav1alpha1.CaptureConfigurationFilters{}
		}
		includeFilterSlice := strings.Split(includeFilter, ",")
		capture.Spec.CaptureConfiguration.Filters.Include = includeFilterSlice
	}
	return capture, nil
}

func getCLICaptureConfig() config.CaptureConfig {
	return config.CaptureConfig{
		CaptureImageVersion:       retinacmd.Version,
		CaptureDebug:              debug,
		CaptureImageVersionSource: captureUtils.VersionSourceCLIVersion,
		CaptureJobNumLimit:        jobNumLimit,
	}
}

func createJobs(kubeClient kubernetes.Interface, capture *retinav1alpha1.Capture) ([]batchv1.Job, error) {
	translator := pkgcapture.NewCaptureToPodTranslator(kubeClient, retinacmd.Logger, getCLICaptureConfig())
	jobs, err := translator.TranslateCaptureToJobs(capture)
	if err != nil {
		return nil, err
	}

	jobsCreated := []batchv1.Job{}
	for _, job := range jobs {
		jobCreated, err := kubeClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		jobsCreated = append(jobsCreated, *jobCreated)
		retinacmd.Logger.Info("Packet capture job is created", zap.String("namespace", namespace), zap.String("capture job", jobCreated.Name))
	}
	return jobsCreated, nil
}

func waitUntilJobsComplete(kubeClient kubernetes.Interface, jobs []batchv1.Job) bool {
	allJobsCompleted := false

	// TODO: let's make the timeout and period to wait for all job to finish configurable.
	var deadline time.Duration = defaultWaitTimeout
	if duration != 0 {
		deadline = duration * 2
	}

	var period time.Duration = defaultWaitPeriod
	// To print less noisy messages, we rely on duration to decide the wait period.
	if period < duration/10 {
		period = duration / 10
	}
	retinacmd.Logger.Info(fmt.Sprintf("Waiting timeout is set to %s", deadline))

	ctx, cancel := context.WithTimeout(context.TODO(), deadline)
	defer cancel()

	wait.JitterUntil(func() {
		jobsCompleted := []string{}
		jobsIncompleted := []string{}

		for _, job := range jobs {
			jobRet, err := kubeClient.BatchV1().Jobs(job.Namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
			if err != nil {
				retinacmd.Logger.Error("Failed to get job", zap.String("namespace", job.Namespace), zap.String("job name", job.Name), zap.Error(err))
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
			retinacmd.Logger.Info("Not all jobs are completed",
				zap.String("namespace", namespace),
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

func deleteJobs(kubeClient kubernetes.Interface, jobs []batchv1.Job) []string {
	jobsFailedtoDelete := []string{}
	for _, job := range jobs {
		// Child pods are preserved by default when jobs are deleted, we need to set propagationPolicy=Background to
		// remove them.
		deletePropagationBackground := metav1.DeletePropagationBackground
		err := kubeClient.BatchV1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationBackground,
		})
		if err != nil {
			jobsFailedtoDelete = append(jobsFailedtoDelete, job.Name)
			retinacmd.Logger.Error("Failed to delete job", zap.String("namespace", job.Namespace), zap.String("job name", job.Name), zap.Error(err))
		}
	}
	return jobsFailedtoDelete
}

func init() {
	capture.AddCommand(createCapture)
	createCapture.Flags().DurationVar(&duration, "duration", time.Minute, "Duration of capturing packets")
	createCapture.Flags().IntVar(&maxSize, "max-size", 100, "Limit the capture file to MB in size which works only for Linux") //nolint:gomnd // default
	createCapture.Flags().IntVar(&packetSize, "packet-size", 0, "Limits the each packet to bytes in size which works only for Linux")
	createCapture.Flags().StringVar(&nodeNames, "node-names", "", "A comma-separated list of node names to select nodes on which the network capture will be performed")
	createCapture.Flags().StringVar(&nodeSelectors, "node-selectors", "", "A comma-separated list of node labels to select nodes on which the network capture will be performed")
	createCapture.Flags().StringVar(&podSelectors, "pod-selectors", "",
		"A comma-separated list of pod labels to select pods on which the network capture will be performed")
	createCapture.Flags().StringVar(&namespaceSelectors, "namespace-selectors", "",
		"A comma-separated list of namespace labels in which to apply the pod-selectors. By default, the pod namespace is specified by the flag namespace")
	createCapture.Flags().StringVar(&hostPath, "host-path", "", "HostPath of the node to store the capture files")
	createCapture.Flags().StringVar(&pvc, "pvc", "", "PersistentVolumeClaim under the specified or default namespace to store capture files")
	createCapture.Flags().StringVar(&blobUpload, "blob-upload", "", "Blob SAS URL with write permission to upload capture files")
	createCapture.Flags().StringVar(&tcpdumpFilter, "tcpdump-filter", "", "Raw tcpdump flags which works only for Linux")
	createCapture.Flags().StringVar(&excludeFilter, "exclude-filter", "", "A comma-separated list of IP:Port pairs that are "+
		"excluded from capturing network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*")
	createCapture.Flags().StringVar(&includeFilter, "include-filter", "", "A comma-separated list of IP:Port pairs that are "+
		"used to filter capture network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*")
	createCapture.Flags().BoolVar(&includeMetadata, "include-metadata", true, "If true, collect static network metadata into capture file")
	createCapture.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to host capture job")
	createCapture.Flags().IntVar(&jobNumLimit, "job-num-limit", 0, "The maximum number of jobs can be created for each capture. 0 means no limit")
	createCapture.Flags().BoolVar(&nowait, "no-wait", true, "Do not wait for the long-running capture job to finish")
	createCapture.Flags().BoolVar(&debug, "debug", false, "When debug is true, a customized retina-agent image, determined by the environment variable RETINA_AGENT_IMAGE, is set")
}
