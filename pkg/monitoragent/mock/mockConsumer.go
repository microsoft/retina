package mock

import "github.com/cilium/cilium/pkg/monitor/agent/consumer"

// Verify interface compliance at compile time
var _ consumer.MonitorConsumer = (*MockConsumer)(nil)

type MockConsumer struct {
	consumer.MonitorConsumer
}

func (m *MockConsumer) NotifyAgentEvent(typ int, message interface{}) {
}
