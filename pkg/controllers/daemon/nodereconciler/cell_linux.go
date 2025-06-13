package nodereconciler

import (
	"os"

	datapath "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/node/types"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Cell = cell.Module(
	"node-controller",
	"Node Controller monitors Node CRUD events",
	cell.Provide(newNodeController),
	// Setting up the node controller with the controller manager
	cell.Invoke(func(l logrus.FieldLogger, nr *NodeReconciler, ctrlManager ctrl.Manager) error {
		l.Info("Setting up node controller with manager")
		if err := nr.SetupWithManager(ctrlManager); err != nil {
			l.Errorf("failed to setup node controller with manager: %v", err)
			return errors.Wrap(err, "failed to setup node controller with manager")
		}
		return nil
	}),
)

type params struct {
	cell.In

	Config  config.RetinaHubbleConfig
	Client  client.Client
	IPCache *ipcache.IPCache
}

func newNodeController(params params) (*NodeReconciler, error) {
	// TODO: pubsub needs retina logger to already be enabled. Currently
	// we are going to do this within infra module, in which during runtime this will throw a nil pointer err.
	// see if we can avoid this?
	opts := log.GetDefaultLogOpts()
	_, err := log.SetupZapLogger(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup logger")
	}
	n := &NodeReconciler{
		Client:      params.Client,
		clusterName: params.Config.ClusterName,
		l:           log.Logger().Named("node-controller"),
		nodes:       make(map[string]types.Node),
		handlers:    make(map[string]datapath.NodeHandler),
		c:           params.IPCache,
		localNodeIP: os.Getenv("NODE_IP"),
	}
	return n, nil
}
