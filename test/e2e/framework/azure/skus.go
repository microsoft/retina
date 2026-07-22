package azure

import "os"

var (
	AgentLinuxSKU    = skuFromEnv("AZURE_AGENT_LINUX_SKU", "Standard_D4s_v3")
	AgentLinuxARMSKU = skuFromEnv("AZURE_AGENT_LINUX_ARM_SKU", "Standard_D4pds_v5")
	AgentWindowsSKU  = skuFromEnv("AZURE_AGENT_WINDOWS_SKU", "Standard_D4ds_v4")
)

func skuFromEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
