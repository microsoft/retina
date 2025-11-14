package shell

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// convertToCapabilities converts a slice of strings to a slice of v1.Capability
func ephemeralContainerForPodDebug(config Config) v1.EphemeralContainer {
	return v1.EphemeralContainer{
		EphemeralContainerCommon: v1.EphemeralContainerCommon{
			Name:  randomRetinaShellContainerName(),
			Image: config.RetinaShellImage,
			Stdin: true,
			TTY:   true,
			SecurityContext: &v1.SecurityContext{
				Capabilities: &v1.Capabilities{
					Drop: []v1.Capability{"ALL"},
					Add:  stringSliceToCapabilities(config.Capabilities),
				},
			},
		},
	}
}

func hostNetworkPodForNodeDebug(config Config, debugPodNamespace, nodeName string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      randomRetinaShellContainerName(),
			Namespace: debugPodNamespace,
		},
		Spec: v1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations:   []v1.Toleration{{Operator: v1.TolerationOpExists}},
			HostNetwork:   true,
			HostPID:       config.HostPID,
			Containers: []v1.Container{
				{
					Name:  "retina-shell",
					Image: config.RetinaShellImage,
					Stdin: true,
					TTY:   true,
					SecurityContext: &v1.SecurityContext{
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
							Add:  stringSliceToCapabilities(config.Capabilities),
						},
					},
				},
			},
		},
	}

	if config.MountHostFilesystem || config.AllowHostFilesystemWrite {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "host-filesystem",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/",
					},
				},
			},
			v1.Volume{
				Name: "run",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/run",
					},
				},
			},
		)
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			v1.VolumeMount{
				Name:      "host-filesystem",
				MountPath: "/host",
				ReadOnly:  !config.AllowHostFilesystemWrite,
			},
			v1.VolumeMount{
				Name:      "run",
				MountPath: "/run",
			},
		)
	}

	if config.AppArmorUnconfined {
		pod.Spec.Containers[0].SecurityContext.AppArmorProfile = &v1.AppArmorProfile{
			Type: v1.AppArmorProfileTypeUnconfined,
		}
	}

	if config.SeccompUnconfined {
		pod.Spec.Containers[0].SecurityContext.SeccompProfile = &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeUnconfined,
		}
	}

	return pod
}

func randomRetinaShellContainerName() string {
	const retinaShellContainerNameRandLen = 5
	return "retina-shell-" + utilrand.String(retinaShellContainerNameRandLen)
}

func stringSliceToCapabilities(ss []string) []v1.Capability {
	caps := make([]v1.Capability, 0, len(ss))
	for _, s := range ss {
		caps = append(caps, v1.Capability(s))
	}
	return caps
}
