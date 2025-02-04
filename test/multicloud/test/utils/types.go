package utils

const (
	ExamplesPath  = "../../examples/"
	RetinaVersion = "v0.0.24"
)

type PodSelector struct {
	Namespace     string
	LabelSelector string
	ContainerName string
}
