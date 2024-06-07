package k8s

import (
	"context"
	"net"
	"os"

	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/node/addressing"
	nodetypes "github.com/cilium/cilium/pkg/node/types"
	"github.com/sirupsen/logrus"
)

type nodeSynchronizer struct {
	l *logrus.Entry
}

func (n *nodeSynchronizer) InitLocalNode(ctx context.Context, ln *node.LocalNode) error {
	if ln == nil {
		n.l.Warn("Local node is nil")
		return nil
	}
	nodeIP := os.Getenv("NODE_IP")
	if nodeIP == "" {
		n.l.Warn("Failed to get NODE_IP")
		return nil
	}
	ln.Node = nodetypes.Node{
		IPAddresses: []nodetypes.Address{
			{
				IP:   net.ParseIP(nodeIP),
				Type: addressing.NodeExternalIP,
			},
		},
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}
	return nil
}

func (n *nodeSynchronizer) SyncLocalNode(context.Context, *node.LocalNodeStore) {
	n.l.Info("SyncLocalNode called")
}
