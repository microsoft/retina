package telemetry

type Perf interface {
	GetMemoryUsage() map[string]string
	GetCPUUsage() (map[string]string, error)
}
