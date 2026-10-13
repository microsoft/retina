//go:build unit

package nodereconciler

import (
	"sync"
	"testing"

	"github.com/cilium/cilium/pkg/node"
	nodetypes "github.com/cilium/cilium/pkg/node/types"
	"github.com/microsoft/retina/pkg/log"
)

// peerHandler mimics hubble's peer handler: every instance reports the same
// constant Name(), the way pkg/hubble/peer/handler.go does. The field is not
// unused: a zero-size struct would give every allocation the same address and
// collapse the identity-keyed handler map to one entry.
type peerHandler struct {
	id int
}

func (*peerHandler) Name() string                                    { return "hubble-peer" }
func (*peerHandler) NodeAdd(nodetypes.Node) error                    { return nil }
func (*peerHandler) NodeUpdate(_, _ nodetypes.Node) error            { return nil }
func (*peerHandler) NodeDelete(nodetypes.Node) error                 { return nil }
func (*peerHandler) AllNodeValidateImplementation()                  {}
func (*peerHandler) NodeValidateImplementation(nodetypes.Node) error { return nil }

func TestSubscribeConcurrentSameNameHandlers(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("setting up logger: %v", err)
	}
	r := &NodeReconciler{
		l:        log.Logger().Named("test"),
		handlers: make(map[node.Handler]struct{}),
		nodes: map[string]nodetypes.Node{
			"node-a": {Name: "node-a"},
			"node-b": {Name: "node-b"},
		},
	}

	const streams = 8
	handlers := make([]*peerHandler, streams)
	var wg sync.WaitGroup
	for i := range streams {
		handlers[i] = &peerHandler{id: i}
		wg.Add(1)
		go func(h *peerHandler) {
			defer wg.Done()
			r.Subscribe(h)
		}(handlers[i])
	}
	wg.Wait()

	if got := len(r.handlers); got != streams {
		t.Fatalf("expected %d registered handlers, got %d (same-Name handlers evicted each other)", streams, got)
	}

	// Unsubscribing one stream must not deafen the others.
	r.Unsubscribe(handlers[0])
	if got := len(r.handlers); got != streams-1 {
		t.Fatalf("expected %d handlers after one Unsubscribe, got %d", streams-1, got)
	}
}
