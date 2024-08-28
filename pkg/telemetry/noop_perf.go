package telemetry

type NoopPerfProfile struct{}

func (n *NoopPerfProfile) GetMemoryUsage() map[string]string {
	return make(map[string]string)
}

func NewNoopPerfProfile() *NoopPerfProfile {
	return &NoopPerfProfile{}
}

func (n *NoopPerfProfile) GetCPUUsage() (map[string]string, error) { //nolint unnamed results are fine
	return make(map[string]string), nil
}
