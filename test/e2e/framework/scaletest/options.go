package scaletest

// Options holds parameters for the scale test
type Options struct {
	KubeconfigPath              string
	LabelsToGetMetrics          map[string]string
	AdditionalTelemetryProperty map[string]string
}
