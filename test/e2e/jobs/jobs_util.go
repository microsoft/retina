package retina

func getCloudApiIP(cloudProvider string) string {
	apiEndpoint := "0.0.0.0"

	switch cloudProvider {
	case "aws":
		apiEndpoint = "10.100.0.1"
	case "azure":
		apiEndpoint = "10.0.0.1"
	default:
		panic("Cloud Provider not supported")
	}

	return apiEndpoint
}
