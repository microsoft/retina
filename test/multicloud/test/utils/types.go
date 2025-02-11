package utils

const (
	ExamplesPath                 = "../../examples/"
	RetinaVersion                = "v0.0.24"
	PrometheusHelmValuesStandard = "../../../../deploy/standard/prometheus/values.yaml"
)

type PodSelector struct {
	Namespace     string
	LabelSelector string
	ContainerName string
}
