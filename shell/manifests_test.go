package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

const testRetinaImage = "retina-shell:v0.0.1"

func TestEphemeralContainerForPodDebug(t *testing.T) {
	ec := ephemeralContainerForPodDebug(Config{RetinaShellImage: testRetinaImage})
	assert.True(t, strings.HasPrefix(ec.Name, "retina-shell-"), "Ephemeral container name does not start with the expected prefix")
	assert.Equal(t, testRetinaImage, ec.Image)
	assert.Equal(t, []v1.Capability{"ALL"}, ec.SecurityContext.Capabilities.Drop)
	assert.Empty(t, ec.SecurityContext.Capabilities.Add)
}

func TestEphemeralContainerForPodDebugWithCapabilities(t *testing.T) {
	ec := ephemeralContainerForPodDebug(Config{
		RetinaShellImage: testRetinaImage,
		Capabilities:     []string{"NET_RAW", "NET_ADMIN"},
	})
	assert.Equal(t, []v1.Capability{"NET_RAW", "NET_ADMIN"}, ec.SecurityContext.Capabilities.Add)
}

func TestHostNetworkPodForNodeDebug(t *testing.T) {
	config := Config{RetinaShellImage: testRetinaImage}
	pod := hostNetworkPodForNodeDebug(config, "kube-system", "node0001")
	assert.True(t, strings.HasPrefix(pod.Name, "retina-shell-"), "Pod name does not start with the expected prefix")
	assert.Equal(t, "kube-system", pod.Namespace)
	assert.Equal(t, "node0001", pod.Spec.NodeName)
	assert.Equal(t, v1.RestartPolicyNever, pod.Spec.RestartPolicy)
	assert.Equal(t, []v1.Toleration{{Operator: v1.TolerationOpExists}}, pod.Spec.Tolerations)
	assert.True(t, pod.Spec.HostNetwork, "Pod does not have host network enabled")
	assert.False(t, pod.Spec.HostPID)
	assert.Len(t, pod.Spec.Containers, 1)
	assert.Equal(t, testRetinaImage, pod.Spec.Containers[0].Image)
	assert.Equal(t, []v1.Capability{"ALL"}, pod.Spec.Containers[0].SecurityContext.Capabilities.Drop)
	assert.Empty(t, pod.Spec.Containers[0].SecurityContext.Capabilities.Add)
	assert.Empty(t, pod.Spec.Volumes)
	assert.Empty(t, pod.Spec.Containers[0].VolumeMounts)
}

func TestHostNetworkPodForNodeDebugWithHostPID(t *testing.T) {
	config := Config{
		RetinaShellImage: testRetinaImage,
		HostPID:          true,
	}
	pod := hostNetworkPodForNodeDebug(config, "kube-system", "node0001")
	assert.True(t, pod.Spec.HostPID, "Pod does not have host PID enabled")
}

func TestHostNetworkPodForNodeDebugWithCapabilities(t *testing.T) {
	config := Config{
		RetinaShellImage: testRetinaImage,
		Capabilities:     []string{"NET_RAW", "NET_ADMIN"},
	}
	pod := hostNetworkPodForNodeDebug(config, "kube-system", "node0001")
	assert.Equal(t, []v1.Capability{"NET_RAW", "NET_ADMIN"}, pod.Spec.Containers[0].SecurityContext.Capabilities.Add)
}

func TestHostNetworkPodForNodeDebugWithMountHostFilesystem(t *testing.T) {
	config := Config{
		RetinaShellImage:    testRetinaImage,
		MountHostFilesystem: true,
	}
	pod := hostNetworkPodForNodeDebug(config, "kube-system", "node0001")
	assert.Len(t, pod.Spec.Volumes, 2)
	assert.Equal(t, "host-filesystem", pod.Spec.Volumes[0].Name)
	assert.Len(t, pod.Spec.Containers[0].VolumeMounts, 2)
	assert.Equal(t, "host-filesystem", pod.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "/host", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Equal(t, "run", pod.Spec.Containers[0].VolumeMounts[1].Name)
	assert.Equal(t, "/run", pod.Spec.Containers[0].VolumeMounts[1].MountPath)
	assert.True(t, pod.Spec.Containers[0].VolumeMounts[0].ReadOnly)
	assert.False(t, pod.Spec.Containers[0].VolumeMounts[1].ReadOnly)
}

func TestHostNetworkPodForNodeDebugWithMountHostFilesystemWithWriteAccess(t *testing.T) {
	config := Config{
		RetinaShellImage:         testRetinaImage,
		AllowHostFilesystemWrite: true,
	}
	pod := hostNetworkPodForNodeDebug(config, "kube-system", "node0001")
	assert.Len(t, pod.Spec.Volumes, 2)
	assert.Equal(t, "host-filesystem", pod.Spec.Volumes[0].Name)
	assert.Len(t, pod.Spec.Containers[0].VolumeMounts, 2)
	assert.Equal(t, "host-filesystem", pod.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "/host", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Equal(t, "run", pod.Spec.Containers[0].VolumeMounts[1].Name)
	assert.Equal(t, "/run", pod.Spec.Containers[0].VolumeMounts[1].MountPath)
	assert.False(t, pod.Spec.Containers[0].VolumeMounts[0].ReadOnly)
	assert.False(t, pod.Spec.Containers[0].VolumeMounts[1].ReadOnly)
}
