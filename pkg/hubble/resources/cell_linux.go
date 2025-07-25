package resources

import (
	"github.com/cilium/hive/cell"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
)

var Cell = cell.Module(
	"resources",
	"Resources for Hubble",
	cell.Provide(NewServiceReconciler),
	cell.Provide(NewCiliumIdentityReconciler),
	cell.Invoke(func(svc *ServiceReconciler, cid *CiliumIdentityReconciler, ctrlManager ctrl.Manager) error {
		if err := svc.SetupWithManager(ctrlManager); err != nil {
			svc.logger.Error("failed to setup service reconciler with manager", zap.Error(err))
			return errors.Wrap(err, "failed to setup service reconciler with manager")
		}
		svc.logger.Info("Service reconciler setup completed")
		if err := cid.SetupWithManager(ctrlManager); err != nil {
			cid.logger.Error("failed to setup cilium identity reconciler with manager", zap.Error(err))
			return errors.Wrap(err, "failed to setup cilium identity reconciler with manager")
		}
		cid.logger.Info("Cilium identity reconciler setup completed")
		return nil
	}),
)
