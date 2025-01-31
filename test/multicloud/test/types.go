package test

const examplesPath = "../examples/"

type PodSelector struct {
	Namespace     string
	LabelSelector string
	ContainerName string
}
