// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/label"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const anyIPOrPort = ""

// CaptureTarget indicates on which the network capture will be performed on a given node.
type CaptureTarget struct {
	// PodIpAddresses indicates the capture is performed on the Pods per their IP addresses.
	PodIpAddresses []string
	// CaptureNodeInterface indicates the capture is performed on the host node interface.
	CaptureNodeInterface bool

	// OS indicates the operating system of the node hosting the target.
	OS string
}

// CaptureTargetsOnNode maps nodes to the targets network capture will be performed.
// On each node, the network capture can be done on either Pod hosted in the node, or the node interface itself.
// When multiple nodes are selected per capture configuration, there'll be multiple jobs created per node.
type CaptureTargetsOnNode map[string]CaptureTarget

func (cton CaptureTargetsOnNode) AddPod(hostname string, ipAddresses []string) {
	if captureTarget, ok := cton[hostname]; !ok {
		cton[hostname] = CaptureTarget{
			PodIpAddresses: ipAddresses,
		}
	} else {
		captureTarget.PodIpAddresses = append(captureTarget.PodIpAddresses, ipAddresses...)
		cton[hostname] = captureTarget
	}
}

func (cton CaptureTargetsOnNode) AddNodeInterface(hostname string) {
	cton[hostname] = CaptureTarget{
		CaptureNodeInterface: true,
	}
}

func (cton CaptureTargetsOnNode) UpdateNodeOS(hostname string, os string) {
	if captureTarget, ok := cton[hostname]; ok {
		captureTarget.OS = os
		cton[hostname] = captureTarget
	}
}

// CaptureToPodTranslator translate the Capture object to a Job.
type CaptureToPodTranslator struct {
	kubeClient           kubernetes.Interface
	jobTemplate          *batchv1.Job
	captureWorkloadImage string

	config config.CaptureConfig

	// Apiserver is unique identifier to identify the cluster in the trace capture.
	Apiserver string

	l *log.ZapLogger
}

// NewCaptureToPodTranslator initializes a CaptureToPodTranslator.
func NewCaptureToPodTranslator(kubeClient kubernetes.Interface, logger *log.ZapLogger, config config.CaptureConfig) *CaptureToPodTranslator {
	captureWorkloadImage := captureUtils.CaptureWorkloadImage(logger, config.CaptureImageVersion, config.CaptureDebug, config.CaptureImageVersionSource)

	captureToPodTranslator := &CaptureToPodTranslator{
		kubeClient:           kubeClient,
		captureWorkloadImage: captureWorkloadImage,
		config:               config,
		l:                    logger,
	}

	apierverURL, err := telemetry.GetK8SApiserverURLFromKubeConfig()
	if err != nil {
		captureToPodTranslator.l.Error("Failed to get apiserver URL", zap.Error(err))
		// TODO(mainred): should we return error here?
		captureToPodTranslator.Apiserver = ""
	} else {
		captureToPodTranslator.Apiserver = apierverURL
	}

	return captureToPodTranslator
}

func (translator *CaptureToPodTranslator) initJobTemplate(ctx context.Context, capture *retinav1alpha1.Capture) error {
	backoffLimit := int32(0)
	// NOTE(mainred): We allow the capture pod to run for at most 30 minutes before being deleted to ensure the output is
	// uploaded, and this happens when the user want to stop a capture on demand by deleting the capture.
	captureTerminationGracePeriodSeconds := int64(1800)
	translator.jobTemplate = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", capture.Name),
			Namespace:    capture.Namespace,
			Labels:       captureUtils.GetJobLabelsFromCaptureName(capture.Name),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      captureUtils.GetContainerLabelsFromCaptureName(capture.Name),
					Namespace:   capture.Namespace,
					Annotations: captureUtils.GetPodAnnotationsFromCapture(capture),
				},
				Spec: corev1.PodSpec{
					HostNetwork:                   true,
					HostIPC:                       true,
					TerminationGracePeriodSeconds: &captureTerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:            captureConstants.CaptureContainername,
							Image:           translator.captureWorkloadImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN", "SYS_ADMIN",
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: telemetry.EnvPodName,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  captureConstants.ApiserverEnvKey,
									Value: translator.Apiserver,
								},
							},
							// Usually, the Capture Pod takes no more than 10m CPU
							// and 10Mi memory. And considering the Capture Pod
							// does not consumes much resources as required by
							// the workload, it's safe to hard-code the resource
							// requests and limits.
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
									// Initial test 128Mi does not always work on Capture on Windows node
									// for OutOfMemoryException.
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
							},
						},
					},

					RestartPolicy: corev1.RestartPolicyNever,

					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: "Exists",
						},
						{
							Effect:   "NoExecute",
							Operator: "Exists",
						},
						{
							Effect:   "NoSchedule",
							Operator: "Exists",
						},
					},
				},
			},
		},
	}

	if capture.Spec.OutputConfiguration.HostPath != nil && *capture.Spec.OutputConfiguration.HostPath != "" {
		translator.l.Info("HostPath is not empty", zap.String("HostPath", *capture.Spec.OutputConfiguration.HostPath))

		captureFolderHostPathType := corev1.HostPathDirectoryOrCreate
		hostPath := *capture.Spec.OutputConfiguration.HostPath
		hostPathVolume := corev1.Volume{
			Name: captureConstants.CaptureHostPathVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostPath,
					Type: &captureFolderHostPathType,
				},
			},
		}
		translator.jobTemplate.Spec.Template.Spec.Volumes = append(translator.jobTemplate.Spec.Template.Spec.Volumes, hostPathVolume)

		hostPathVolumeMount := corev1.VolumeMount{
			Name:      captureConstants.CaptureHostPathVolumeName,
			MountPath: hostPath,
		}
		translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts, hostPathVolumeMount)
	}

	if capture.Spec.OutputConfiguration.BlobUpload != nil && *capture.Spec.OutputConfiguration.BlobUpload != "" {
		translator.l.Info("BlobUpload is not empty")
		secret, err := translator.kubeClient.CoreV1().Secrets(capture.Namespace).Get(ctx, *capture.Spec.OutputConfiguration.BlobUpload, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			err := SecretNotFoundError{SecretName: *capture.Spec.OutputConfiguration.BlobUpload, Namespace: capture.Namespace}
			translator.l.Error(err.Error())
			return err
		}
		if err != nil {
			translator.l.Error("Failed to get secrets for Capture", zap.Error(err), zap.String("CaptureName", capture.Name), zap.String("secretName", *capture.Spec.OutputConfiguration.BlobUpload))
			return err
		}

		secretVolume := corev1.Volume{
			Name: *capture.Spec.OutputConfiguration.BlobUpload,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		}
		translator.jobTemplate.Spec.Template.Spec.Volumes = append(translator.jobTemplate.Spec.Template.Spec.Volumes, secretVolume)

		secretVolumeMount := corev1.VolumeMount{
			Name:      *capture.Spec.OutputConfiguration.BlobUpload,
			ReadOnly:  true,
			MountPath: captureConstants.CaptureOutputLocationBlobUploadSecretPath,
		}

		translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts, secretVolumeMount)
	}

	if capture.Spec.OutputConfiguration.S3Upload != nil && capture.Spec.OutputConfiguration.S3Upload.SecretName != "" {
		translator.l.Info("S3Upload is not empty")
		secret, err := translator.kubeClient.CoreV1().Secrets(capture.Namespace).Get(ctx, capture.Spec.OutputConfiguration.S3Upload.SecretName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			err := SecretNotFoundError{SecretName: capture.Spec.OutputConfiguration.S3Upload.SecretName, Namespace: capture.Namespace}
			translator.l.Error(err.Error())
			return err
		}
		if err != nil {
			translator.l.Error("Failed to get secrets for Capture", zap.Error(err), zap.String("CaptureName", capture.Name), zap.String("secretName", capture.Spec.OutputConfiguration.S3Upload.SecretName))
			return fmt.Errorf("failed to get secrets for Capture: %w", err)
		}

		secretVolume := corev1.Volume{
			Name: capture.Spec.OutputConfiguration.S3Upload.SecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		}
		translator.jobTemplate.Spec.Template.Spec.Volumes = append(translator.jobTemplate.Spec.Template.Spec.Volumes, secretVolume)

		secretVolumeMount := corev1.VolumeMount{
			Name:      capture.Spec.OutputConfiguration.S3Upload.SecretName,
			ReadOnly:  true,
			MountPath: captureConstants.CaptureOutputLocationS3UploadSecretPath,
		}

		translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts, secretVolumeMount)
	}

	if capture.Spec.OutputConfiguration.PersistentVolumeClaim != nil && *capture.Spec.OutputConfiguration.PersistentVolumeClaim != "" {
		translator.l.Info("PersistentVolumeClaim is not empty", zap.String("PersistentVolumeClaim", *capture.Spec.OutputConfiguration.PersistentVolumeClaim))

		_, err := translator.kubeClient.CoreV1().PersistentVolumeClaims(capture.Namespace).Get(ctx, *capture.Spec.OutputConfiguration.PersistentVolumeClaim, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pvc %s/%s", capture.Namespace, *capture.Spec.OutputConfiguration.PersistentVolumeClaim)
		}
		pvcVolume := corev1.Volume{
			Name: captureConstants.CapturePVCVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: *capture.Spec.OutputConfiguration.PersistentVolumeClaim,
				},
			},
		}
		translator.jobTemplate.Spec.Template.Spec.Volumes = append(translator.jobTemplate.Spec.Template.Spec.Volumes, pvcVolume)

		pvcVolumeMountPath := captureConstants.PersistentVolumeClaimVolumeMountPathLinux
		pvcVolumeMount := corev1.VolumeMount{
			Name:      captureConstants.CapturePVCVolumeName,
			MountPath: pvcVolumeMountPath,
		}
		translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts = append(translator.jobTemplate.Spec.Template.Spec.Containers[0].VolumeMounts, pvcVolumeMount)
	}
	return nil
}

// validateNoRunningWindowsCapture checks if there's any running capture jobs on the Windows to deploy capture.
// Windows node allows only one capture job running for only one tracing session is allowed at one time.
func (translator *CaptureToPodTranslator) validateNoRunningWindowsCapture(ctx context.Context, captureTargetOnNode *CaptureTargetsOnNode) error {
	// map is easy for removing duplication.
	windowsNodesRunningCapture := map[string]struct{}{}
	capturePodSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{label.AppLabel: captureConstants.CaptureAppname},
	}
	labelSelector, _ := labels.Parse(metav1.FormatLabelSelector(capturePodSelector))
	podListOpt := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	podList, err := translator.kubeClient.CoreV1().Pods("").List(ctx, podListOpt)
	if err != nil {
		translator.l.Error("Failed to list capture Pods", zap.String("podSelector", capturePodSelector.String()), zap.Error(err))
		return err
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodPending && pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if (*captureTargetOnNode)[pod.Spec.NodeName].OS == "windows" {
			windowsNodesRunningCapture[pod.Spec.NodeName] = struct{}{}
		}
	}

	if len(windowsNodesRunningCapture) != 0 {
		windowsNodeToPrint := []string{}
		for windowsNode := range windowsNodesRunningCapture {
			windowsNodeToPrint = append(windowsNodeToPrint, windowsNode)
		}
		return fmt.Errorf("Windows node allows only one capture job running at one time, but there is already in-progress capture job on Windows nodes %s", windowsNodeToPrint)
	}
	return nil
}

func (translator *CaptureToPodTranslator) TranslateCaptureToJobs(ctx context.Context, capture *retinav1alpha1.Capture) ([]*batchv1.Job, error) {
	if err := translator.validateCapture(capture); err != nil {
		return nil, err
	}

	if err := translator.initJobTemplate(ctx, capture); err != nil {
		return nil, err
	}
	captureTargetOnNode, err := translator.CalculateCaptureTargetsOnNode(ctx, capture.Spec.CaptureConfiguration.CaptureTarget)
	if err != nil {
		return nil, err
	}

	jobPodEnv, err := translator.ObtainCaptureJobPodEnv(*capture)
	if err != nil {
		return nil, err
	}

	jobs, err := translator.renderJob(captureTargetOnNode, jobPodEnv)
	if err != nil {
		return nil, err
	}

	if translator.config.CaptureJobNumLimit != 0 && len(jobs) > translator.config.CaptureJobNumLimit {
		return nil, CaptureJobNumExceedLimitError{CurrentNum: len(jobs), Limit: translator.config.CaptureJobNumLimit}
	}

	return jobs, nil
}

func (translator *CaptureToPodTranslator) renderJob(captureTargetOnNode *CaptureTargetsOnNode, envCommon map[string]string) ([]*batchv1.Job, error) {
	if len(*captureTargetOnNode) == 0 {
		return nil, fmt.Errorf("no nodes are selected")
	}

	stringTimestamp := translator.jobTemplate.Spec.Template.ObjectMeta.Annotations[captureConstants.CaptureTimestampAnnotationKey]
	captureTimestamp, err := file.StringToTime(stringTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse capture start timestamp: %w", err)
	}

	fmt.Println("#########################")
	fmt.Println("Expected Capture Files")
	fmt.Println("#########################")

	jobs := make([]*batchv1.Job, 0, len(*captureTargetOnNode))
	for nodeName, target := range *captureTargetOnNode {
		jobEnv := make(map[string]string, len(envCommon))
		for k, v := range envCommon {
			jobEnv[k] = v
		}
		job := translator.jobTemplate.DeepCopy()
		captureFilename := &file.CaptureFilename{
			CaptureName:    envCommon[captureConstants.CaptureNameEnvKey],
			NodeHostname:   nodeName,
			StartTimestamp: captureTimestamp,
		}
		job.Spec.Template.ObjectMeta.Annotations[captureConstants.CaptureFilenameAnnotationKey] = captureFilename.String()

		fmt.Printf("%s.tar.gz\n", captureFilename.String())

		job.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      corev1.LabelHostname,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{nodeName},
								},
							},
						},
					},
				},
			},
		}

		if target.OS == "linux" {
			// tcpdump requires run as a root for Linux, while for pods deployed on Windows node, there's an ongoing
			// issue that causes pods to not start on Windows.
			// ref: https://github.com/kubernetes/kubernetes/issues/102849
			rootUser := int64(0)
			job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &rootUser
			job.Spec.Template.Spec.Containers[0].Command = []string{captureConstants.CaptureContainerEntrypoint}

			delete(jobEnv, captureConstants.NetshFilterEnvKey)
			// Update tcpdumpfilter to include target POD IP address.
			if updatedTcpdumpFilter := updateTcpdumpFilterWithPodIPAddress(target.PodIpAddresses, jobEnv[captureConstants.TcpdumpFilterEnvKey]); len(updatedTcpdumpFilter) != 0 {
				jobEnv[captureConstants.TcpdumpFilterEnvKey] = updatedTcpdumpFilter
			}
		} else {
			containerAdministrator := "NT AUTHORITY\\SYSTEM"
			useHostProcess := true
			job.Spec.Template.Spec.Containers[0].SecurityContext.WindowsOptions = &corev1.WindowsSecurityContextOptions{
				HostProcess:   &useHostProcess,
				RunAsUserName: &containerAdministrator,
			}
			job.Spec.Template.Spec.Containers[0].Command = []string{captureConstants.CaptureContainerEntrypointWin}

			// Update to pvc mount path for Windows pods.
			for i, volumeMount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
				if volumeMount.MountPath == captureConstants.PersistentVolumeClaimVolumeMountPathLinux {
					job.Spec.Template.Spec.Containers[0].VolumeMounts[i].MountPath = captureConstants.PersistentVolumeClaimVolumeMountPathWin
					break
				}
			}

			delete(jobEnv, captureConstants.TcpdumpFilterEnvKey)
			if netshFilter := getNetshFilterWithPodIPAddress(target.PodIpAddresses); len(netshFilter) != 0 {
				jobEnv[captureConstants.NetshFilterEnvKey] = netshFilter
			}
		}

		for k, v := range jobEnv {
			job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: k, Value: v})
		}
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: captureConstants.NodeHostNameEnvKey, Value: nodeName})
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: captureConstants.CaptureStartTimestampEnvKey, Value: stringTimestamp})

		jobs = append(jobs, job)
	}

	fmt.Println("\nNote: The file(s) may not be created if the capture job(s) fail prematurely.")
	fmt.Println("#########################")

	return jobs, nil
}

func updateTcpdumpFilterWithPodIPAddress(podIPAddresses []string, tcpdumpFilter string) string {
	if len(podIPAddresses) == 0 {
		return tcpdumpFilter
	}
	podIPFilterArray := []string{}
	for _, podIPAddress := range podIPAddresses {
		podIPFilterArray = append(podIPFilterArray, fmt.Sprintf("host %s", podIPAddress))
	}

	// Use logic OR to capture network traffic when any of the Pod IP Address meets.
	filterGroup := fmt.Sprintf("(%s)", strings.Join(podIPFilterArray, " or "))

	if len(tcpdumpFilter) == 0 {
		return filterGroup
	}
	return fmt.Sprintf("%s or %s", tcpdumpFilter, filterGroup)
}

func getNetshFilterWithPodIPAddress(podIPAddresses []string) string {
	if len(podIPAddresses) == 0 {
		return ""
	}

	// netsh accepts multiple IP address as filters for the trace capture.
	// Example: IPv4.Address=(157.59.136.1,157.59.136.11)
	// Please check `netsh trace show capturefilterhelp` for the detail.
	podIPv4Addresses := []string{}
	podIPv6Addresses := []string{}
	for _, podIPAddress := range podIPAddresses {
		// check ipv4 or ipv6
		parsedIP := net.ParseIP(podIPAddress)
		if parsedIP == nil {
			continue
		}
		if parsedIP.To4() != nil {
			podIPv4Addresses = append(podIPv4Addresses, podIPAddress)
		} else {
			podIPv6Addresses = append(podIPv6Addresses, podIPAddress)
		}
	}

	var podIPv4FilterGroup string
	if len(podIPv4Addresses) != 0 {
		podIPv4FilterArray := strings.Join(podIPv4Addresses, ",")
		podIPv4FilterGroup = fmt.Sprintf("IPv4.Address=(%s)", podIPv4FilterArray)
	}

	var podIPv6FilterGroup string
	if len(podIPv6Addresses) != 0 {
		podIPv6FilterArray := strings.Join(podIPv6Addresses, ",")
		podIPv6FilterGroup = fmt.Sprintf("IPv6.Address=(%s)", podIPv6FilterArray)
	}

	JoinedFilterGroups := strings.Join([]string{podIPv4FilterGroup, podIPv6FilterGroup}, " ")
	return strings.Trim(JoinedFilterGroups, " ")
}

// validateTargetSelector validate target selectors defined in the capture.
func (translator *CaptureToPodTranslator) validateTargetSelector(captureTarget retinav1alpha1.CaptureTarget) error {
	// When NamespaceSelector is nil while PodSelector is specified, the namespace will be determined by capture.Namespace.
	if captureTarget.NodeSelector == nil && captureTarget.PodSelector == nil {
		return fmt.Errorf("Neither NodeSelector nor NamespaceSelector&PodSelector is set.")
	}

	if captureTarget.NodeSelector != nil && (captureTarget.NamespaceSelector != nil || captureTarget.PodSelector != nil) {
		return fmt.Errorf("NodeSelector is not compatible with NamespaceSelector&PodSelector. Please use one or the other.")
	}

	return nil
}

func (translator *CaptureToPodTranslator) validateCapture(capture *retinav1alpha1.Capture) error {
	if err := translator.validateTargetSelector(capture.Spec.CaptureConfiguration.CaptureTarget); err != nil {
		return err
	}

	// TODO(mainred): do we need to set a limitation to the capture file size anyway?
	if capture.Spec.CaptureConfiguration.CaptureOption.Duration == nil && capture.Spec.CaptureConfiguration.CaptureOption.MaxCaptureSize == nil {
		return fmt.Errorf("Neither duration nor maxCaptureSize is set to stop the capture")
	}

	if capture.Spec.OutputConfiguration.BlobUpload == nil &&
		capture.Spec.OutputConfiguration.HostPath == nil &&
		capture.Spec.OutputConfiguration.PersistentVolumeClaim == nil &&
		capture.Spec.OutputConfiguration.S3Upload == nil {
		return fmt.Errorf("At least one output configuration should be set")
	}
	return nil
}

func (translator *CaptureToPodTranslator) getCaptureTargetsOnNode(ctx context.Context, captureTarget retinav1alpha1.CaptureTarget) (*CaptureTargetsOnNode, error) {
	var err error
	captureTargetsOnNode := &CaptureTargetsOnNode{}
	if captureTarget.NodeSelector != nil {
		if captureTargetsOnNode, err = translator.calculateCaptureTargetsByNodeSelector(ctx, captureTarget); err != nil {
			return nil, err
		}
	}
	if captureTarget.PodSelector != nil {
		if captureTargetsOnNode, err = translator.calculateCaptureTargetsByPodSelector(ctx, captureTarget); err != nil {
			return nil, err
		}
	}

	if len(*captureTargetsOnNode) == 0 {
		return nil, fmt.Errorf("no targets are selected by node selector or pod selector")
	}
	return captureTargetsOnNode, nil
}

func (translator *CaptureToPodTranslator) updateCaptureTargetsOSOnNode(ctx context.Context, captureTargetsOnNode *CaptureTargetsOnNode) error {
	nodeNames := []string{}
	for nodeName := range *captureTargetsOnNode {
		nodeNames = append(nodeNames, nodeName)
	}
	nodeLabelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      corev1.LabelHostname,
			Operator: metav1.LabelSelectorOpIn,
			Values:   nodeNames,
		}},
	}
	labelSelector, _ := labels.Parse(metav1.FormatLabelSelector(&nodeLabelSelector))

	nodeListOpt := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	nodeList, err := translator.kubeClient.CoreV1().Nodes().List(ctx, nodeListOpt)
	if err != nil {
		return err
	}
	for _, node := range nodeList.Items {
		captureTargetsOnNode.UpdateNodeOS(node.Name, node.Labels[corev1.LabelOSStable])
	}

	return nil
}

// CalculateCaptureTargetsOnNode returns capture target on each node.
func (translator *CaptureToPodTranslator) CalculateCaptureTargetsOnNode(ctx context.Context, captureTarget retinav1alpha1.CaptureTarget) (*CaptureTargetsOnNode, error) {
	if err := translator.validateTargetSelector(captureTarget); err != nil {
		return nil, err
	}

	captureTargetsOnNode, err := translator.getCaptureTargetsOnNode(ctx, captureTarget)
	if err != nil {
		return nil, err
	}

	if err := translator.updateCaptureTargetsOSOnNode(ctx, captureTargetsOnNode); err != nil {
		return nil, err
	}

	if err := translator.validateNoRunningWindowsCapture(ctx, captureTargetsOnNode); err != nil {
		return nil, err
	}
	return captureTargetsOnNode, nil
}

func (translator *CaptureToPodTranslator) calculateCaptureTargetsByNodeSelector(ctx context.Context, captureTarget retinav1alpha1.CaptureTarget) (*CaptureTargetsOnNode, error) {
	captureTargetOnNode := &CaptureTargetsOnNode{}
	labelSelector, err := labels.Parse(metav1.FormatLabelSelector(captureTarget.NodeSelector))
	if err != nil {
		translator.l.Error("Failed to parse node selector to label", zap.String("nodeSelector", captureTarget.NodeSelector.String()), zap.Error(err))
		return nil, err
	}
	nodeListOpt := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	nodeList, err := translator.kubeClient.CoreV1().Nodes().List(ctx, nodeListOpt)
	if err != nil {
		translator.l.Error("Failed to list node", zap.String("nodeSelector", fmt.Sprint(captureTarget.NodeSelector.String())), zap.Error(err))
		return nil, err
	}
	for _, node := range nodeList.Items {
		captureTargetOnNode.AddNodeInterface(node.Name)
	}
	return captureTargetOnNode, nil
}

func (translator *CaptureToPodTranslator) calculateCaptureTargetsByPodSelector(ctx context.Context, captureTarget retinav1alpha1.CaptureTarget) (*CaptureTargetsOnNode, error) {
	captureTargetOnNode := &CaptureTargetsOnNode{}
	nsList := &corev1.NamespaceList{Items: []corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: corev1.NamespaceDefault,
			},
		},
	}}

	if captureTarget.NamespaceSelector != nil {
		var err error
		labelSelector, err := labels.Parse(metav1.FormatLabelSelector(captureTarget.NamespaceSelector))
		if err != nil {
			translator.l.Error("PersistentVolumeClaim is not empty", zap.String("namespaceSelector", captureTarget.NamespaceSelector.String()), zap.Error(err))
			return nil, err
		}
		nsListOpt := metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		}
		nsList, err = translator.kubeClient.CoreV1().Namespaces().List(ctx, nsListOpt)
		if err != nil {
			translator.l.Error("Failed to list Namespace", zap.String("namespaceSelector", captureTarget.NamespaceSelector.String()), zap.Error(err))
			return nil, err
		}
	}

	for _, ns := range nsList.Items {
		labelSelector, _ := labels.Parse(metav1.FormatLabelSelector(captureTarget.PodSelector))
		podListOpt := metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		}
		podList, err := translator.kubeClient.CoreV1().Pods(ns.Name).List(ctx, podListOpt)
		if err != nil {
			translator.l.Error("Failed to list Pod.", zap.String("podSelector", captureTarget.PodSelector.String()), zap.Error(err))
			return nil, err
		}
		for _, pod := range podList.Items {
			// TODO: Need to consider the status when pod has no ip address assigned.
			// We want to include all the ip addresses assigned to the Pod in case the Pod is selected.
			// This may happen when a pod is allocated with IPv4 and IPv6 addresses.
			// And Pod.Status.PodIPs must include pod.Status.PodIP.
			podIPs := []string{}
			for _, podIP := range pod.Status.PodIPs {
				podIPs = append(podIPs, podIP.IP)
			}
			captureTargetOnNode.AddPod(pod.Spec.NodeName, podIPs)
		}
	}

	return captureTargetOnNode, nil
}

// For tcpdump, we put each filter(ip:port) into parentheses, and all include filters will be grouped in parentheses,
// same case for exclude filters, finally the filters overall will be like:
// ((include1) or (include2)) and not ((exclude1) or (exclude2))
func tcpdumpFiltersFromIncludeAndExcludeFilters(includeIPPortsFilters, excludeIPPortsFilters map[string][]string) string {
	getFilterGroupFunc := func(ipPortsFilters map[string][]string) string {
		if len(ipPortsFilters) == 0 {
			return ""
		}
		ips := make([]string, 0)

		for ip := range ipPortsFilters {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		var filterArray []string
		for _, ip := range ips {
			ports := ipPortsFilters[ip]
			sort.Strings(ports)
			var filter string
			if ip == anyIPOrPort {
				for _, port := range ports {
					filter = fmt.Sprintf("(port %s)", port)
					filterArray = append(filterArray, filter)
				}
				continue
			}
			if len(ports) == 1 && ports[0] == anyIPOrPort {
				filter = fmt.Sprintf("(host %s)", ip)
				filterArray = append(filterArray, filter)
				continue
			}
			for _, port := range ports {
				filter = fmt.Sprintf("(host %s and port %s)", ip, port)
				filterArray = append(filterArray, filter)
			}
		}
		if len(filterArray) == 0 {
			return ""
		}
		filterGroup := fmt.Sprintf("(%s)", strings.Join(filterArray, " or "))
		return filterGroup
	}

	includeFilterGroup := getFilterGroupFunc(includeIPPortsFilters)
	excludeFilterGroup := getFilterGroupFunc(excludeIPPortsFilters)

	if len(includeFilterGroup) == 0 && len(excludeFilterGroup) == 0 {
		return ""
	}
	if len(includeFilterGroup) != 0 && len(excludeFilterGroup) != 0 {
		return fmt.Sprintf("%s and not %s", includeFilterGroup, excludeFilterGroup)
	}
	if len(includeFilterGroup) != 0 && len(excludeFilterGroup) == 0 {
		return includeFilterGroup
	}
	if len(includeFilterGroup) == 0 && len(excludeFilterGroup) != 0 {
		return fmt.Sprintf("not %s", excludeFilterGroup)
	}
	return ""
}

// parseIncludeAndExcludeFilters returns organized include and exclude IP:Port filters.
func parseIncludeAndExcludeFilters(filters *retinav1alpha1.CaptureConfigurationFilters) (map[string][]string, map[string][]string, error) {
	if filters == nil {
		return nil, nil, nil
	}
	var includeIPPortFilters, excludeIPPortFilters map[string][]string
	var err error
	if len(filters.Include) != 0 {
		if includeIPPortFilters, err = filterToIPPortsMap(filters.Include); err != nil {
			return nil, nil, err
		}
	}
	if len(filters.Exclude) != 0 {
		if excludeIPPortFilters, err = filterToIPPortsMap(filters.Exclude); err != nil {
			return nil, nil, err
		}
	}
	return includeIPPortFilters, excludeIPPortFilters, nil
}

// filterToIPPortsMap maps ip address to a list of port sharing this ip address.
// If the key(ip address) is empty, ports, in combination with any ip address, in the list will be included/excluded.
// If the values(ports) contains only one empty string, all the ports together with that key(ip address) will be included/excluded.
func filterToIPPortsMap(filters []string) (map[string][]string, error) {
	// map[string]struct{}{} simplifies duplication removal and seek.
	filterIPPorts := map[string]map[string]struct{}{}
	for _, filter := range filters {
		if len(filter) == 0 {
			continue
		}
		var ipAddressFilter, portFilter string
		// The filter may include both ip and port with format ip:port with wildcard supported, or just port or
		// ip address.
		if strings.Contains(filter, ":") {
			ipPort := strings.Split(filter, ":")
			ipAddressFilter, portFilter = ipPort[0], ipPort[1]
			if ipPort[0] == "*" {
				ipAddressFilter = anyIPOrPort
			}
			if ipPort[1] == "*" {
				portFilter = anyIPOrPort
			}
		} else {
			// For simplicity, we treat string including "." as IP address, or port otherwise, which will finally go
			// against format validation.
			if strings.Contains(filter, ".") {
				ipAddressFilter = filter
			} else {
				portFilter = filter
			}
		}

		// Validate the format of ip address and port.
		if len(ipAddressFilter) != 0 {
			if addr := net.ParseIP(ipAddressFilter); len(addr) == 0 {
				return nil, fmt.Errorf("invalid filter %s", filter)
			}
		}
		if len(portFilter) != 0 {
			portInt, err := strconv.Atoi(portFilter)
			if err != nil || (portInt > 65535 || portInt < 0) {
				return nil, fmt.Errorf("invalid filter %s", filter)
			}
		}

		// If any(* or empty) ip address is specified together with a port, we should ignore the other ip:this_port
		// combinations and keep only *:this_port.
		if ipAddressFilter == anyIPOrPort {
			for _, port := range filterIPPorts {
				delete(port, portFilter)
			}
			if _, ok := filterIPPorts[anyIPOrPort]; ok {
				filterIPPorts[anyIPOrPort][portFilter] = struct{}{}
			} else {
				filterIPPorts[anyIPOrPort] = map[string]struct{}{portFilter: {}}
			}
			continue
		}
		// If any(* or empty) port is specified with an ip address, we should ignore the other ports with combination
		// with this ip address, and the final combination for this ip address will be this_ipaddress:*.
		if portFilter == anyIPOrPort {
			filterIPPorts[ipAddressFilter] = map[string]struct{}{
				anyIPOrPort: {},
			}
			continue
		}
		// Otherwise, we'll initialize the port list to this ip address or append the port to the list.
		_, ok := filterIPPorts[ipAddressFilter]
		if !ok {
			filterIPPorts[ipAddressFilter] = map[string]struct{}{portFilter: {}}
		} else {
			filterIPPorts[ipAddressFilter][portFilter] = struct{}{}
		}

	}

	filterIPPortsMap := map[string][]string{}
	for ip, portList := range filterIPPorts {
		filterIPPortsMap[ip] = []string{}
		for port := range portList {
			filterIPPortsMap[ip] = append(filterIPPortsMap[ip], port)
		}
	}

	return filterIPPortsMap, nil
}

func (translator *CaptureToPodTranslator) obtainTcpdumpFilters(captureConfig retinav1alpha1.CaptureConfiguration) (string, error) {
	if captureConfig.Filters == nil && captureConfig.TcpdumpFilter == nil && captureConfig.CaptureOption.PacketSize == nil {
		return "", nil
	}
	includeIPPortFilters, excludeIPPortFilters, err := parseIncludeAndExcludeFilters(captureConfig.Filters)
	if err != nil {
		return "", err
	}

	tcpdumpFilter := tcpdumpFiltersFromIncludeAndExcludeFilters(includeIPPortFilters, excludeIPPortFilters)
	if len(tcpdumpFilter) != 0 {
		translator.l.Info("Get the parsed filter from include and include filters",
			zap.String("parsed filter", tcpdumpFilter),
			zap.String("Include filters", strings.Join(captureConfig.Filters.Include, ",")),
			zap.String("Exclude filters", strings.Join(captureConfig.Filters.Exclude, ",")),
		)
	}

	translator.l.Info(fmt.Sprintf("The Parsed tcpdump filter is %q", tcpdumpFilter))
	return tcpdumpFilter, nil
}

func (translator *CaptureToPodTranslator) obtainCaptureOutputEnv(outputConfiguration retinav1alpha1.OutputConfiguration) (map[captureConstants.CaptureOutputLocationEnvKey]string, error) {
	outputEnv := map[captureConstants.CaptureOutputLocationEnvKey]string{}
	if outputConfiguration.HostPath != nil {
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyHostPath] = *outputConfiguration.HostPath
	}
	if outputConfiguration.PersistentVolumeClaim != nil {
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim] = *outputConfiguration.PersistentVolumeClaim
	}
	if outputConfiguration.S3Upload != nil {
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyS3Endpoint] = outputConfiguration.S3Upload.Endpoint
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyS3Region] = outputConfiguration.S3Upload.Region
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyS3Bucket] = outputConfiguration.S3Upload.Bucket
		outputEnv[captureConstants.CaptureOutputLocationEnvKeyS3Path] = outputConfiguration.S3Upload.Path
	}

	if len(outputEnv) == 0 && (outputConfiguration.BlobUpload == nil || *outputConfiguration.BlobUpload == "") {
		return nil, fmt.Errorf("need to specify at least one outputConfiguration.")
	}

	return outputEnv, nil
}

// obtainCaptureOptionEnv translates CaptureOption to Environment variables to capture job Pod.
func (translator *CaptureToPodTranslator) obtainCaptureOptionEnv(option retinav1alpha1.CaptureOption) (map[string]string, error) {
	outputEnv := map[string]string{}
	if option.Duration != nil {
		outputEnv[captureConstants.CaptureDurationEnvKey] = option.Duration.Duration.String()
	}
	if option.MaxCaptureSize != nil {
		outputEnv[captureConstants.CaptureMaxSizeEnvKey] = strconv.Itoa(*option.MaxCaptureSize)
	}
	if len(option.Interfaces) > 0 {
		outputEnv[captureConstants.CaptureInterfacesEnvKey] = strings.Join(option.Interfaces, ",")
	}
	return outputEnv, nil
}

// ObtainCaptureJobPodEnv translates Capture object to Environment variables to capture job Pod.
func (translator *CaptureToPodTranslator) ObtainCaptureJobPodEnv(capture retinav1alpha1.Capture) (map[string]string, error) {
	jobPodEnv := map[string]string{}

	captureOutputEnv, err := translator.obtainCaptureOutputEnv(capture.Spec.OutputConfiguration)
	if err != nil {
		return nil, err
	}
	for key, val := range captureOutputEnv {
		jobPodEnv[string(key)] = val
	}

	captureOptionEnv, err := translator.obtainCaptureOptionEnv(capture.Spec.CaptureConfiguration.CaptureOption)
	if err != nil {
		return nil, err
	}
	for key, val := range captureOptionEnv {
		jobPodEnv[string(key)] = val
	}

	tcpdumpFilter, err := translator.obtainTcpdumpFilters(capture.Spec.CaptureConfiguration)
	if err != nil {
		return nil, err
	}
	if len(tcpdumpFilter) != 0 {
		jobPodEnv[captureConstants.TcpdumpFilterEnvKey] = tcpdumpFilter
	}

	if capture.Spec.CaptureConfiguration.CaptureOption.PacketSize != nil {
		jobPodEnv[captureConstants.PacketSizeEnvKey] = strconv.Itoa(*capture.Spec.CaptureConfiguration.CaptureOption.PacketSize)
	}

	if capture.Spec.CaptureConfiguration.TcpdumpFilter != nil {
		jobPodEnv[captureConstants.TcpdumpRawFilterEnvKey] = *capture.Spec.CaptureConfiguration.TcpdumpFilter
	}

	if len(capture.Name) != 0 {
		jobPodEnv[captureConstants.CaptureNameEnvKey] = capture.Name
	}
	jobPodEnv[captureConstants.IncludeMetadataEnvKey] = strconv.FormatBool(capture.Spec.CaptureConfiguration.IncludeMetadata)

	// TODO: more env to be added
	return jobPodEnv, nil
}
