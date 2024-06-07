package mock

import (
	"github.com/cilium/cilium/pkg/monitor/agent/listener"
	"github.com/cilium/cilium/pkg/monitor/payload"
)

// Verify interface compliance at compile time
var _ listener.MonitorListener = (*MockListener)(nil)

type MockListener struct {
	listener.MonitorListener

	Unsupported bool
}

func (m *MockListener) Version() listener.Version {
	if m.Unsupported {
		return listener.VersionUnsupported
	}
	return listener.Version1_2
}

func (m *MockListener) Enqueue(pl *payload.Payload) {
}

func (m *MockListener) Close() {
}
