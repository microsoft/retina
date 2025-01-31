package test

type PodSpec struct {
	// The name of the pod
	Name string `json:"name"`
	// The namespace of the pod
	Namespace string `json:"namespace"`
	// The label selector for the pod
	LabelSelector string `json:"label_selector"`
	// The container name
	ContainerName string `json:"container_name"`
}
