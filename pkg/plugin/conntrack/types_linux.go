package conntrack

import (
	"sync"

	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
)

const (
	Name api.PluginName = "conntrack"
	TCP  uint8          = 6  // Transmission Control Protocol
	UDP  uint8          = 17 // User Datagram Protocol
	// Hardcoded pod CIDR and service CIDR for now, we should be getting this via the pod's environment variables
	PodCIDR     string = "192.168.0.0/16"
	ServiceCIDR string = "10.0.0.0/16"
)

type tcpStateMap struct {
	// Map of TCP states to their respective counters
	// Key: TCP state, Value: Counter
	state map[uint8]float64
	mu    sync.Mutex
}

func newTCPStateMap() *tcpStateMap {
	return &tcpStateMap{
		state: make(map[uint8]float64),
	}
}

func (t *tcpStateMap) inc(state uint8) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.state[state]; !ok {
		t.state[state] = 0
	}
	t.state[state]++
}

func (t *tcpStateMap) updateMetrics() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for state, count := range t.state {
		// Update metrics
		metrics.TCPStateGauge.WithLabelValues(TCPState(state)).Set(count)
		// Reset counter
		t.state[state] = 0
	}
}
