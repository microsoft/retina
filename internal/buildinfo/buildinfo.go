package buildinfo

// These variables will be populate by the Go compiler via
// the -ldflags, which insert dynamic information
// into the binary at build time
var (
	// ApplicationInsightsID is the instrumentation key for Azure Application Insights
	// It is set during the build process using the -ldflags flag
	// If it is set, the application will send telemetry to the corresponding Application Insights resource.
	ApplicationInsightsID string
	Version               string
	RetinaAgentImageName  = "ghcr.io/microsoft/retina/retina-agent"
)
