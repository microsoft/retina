package hubble

import (
	"github.com/cilium/hive/cell"
	"github.com/cilium/workerpool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var Cell = cell.Module(
	"retina-hubble",
	"Retina-Hubble runs a Hubble server and observer within the Retina agent",
	cell.Provide(newRetinaHubble),
	cell.Invoke(func(l logrus.FieldLogger, lifecycle cell.Lifecycle, rh *RetinaHubble) {
		var wp *workerpool.WorkerPool
		lifecycle.Append(cell.Hook{
			OnStart: func(cell.HookContext) error {
				wp = workerpool.New(1)
				rh.log.Info("Starting Retina-Hubble")
				if err := wp.Submit("retina-hubble", rh.launchWithDefaultOptions); err != nil {
					rh.log.Fatalf("failed to submit retina-hubble to workerpool: %s", err)
					return errors.Wrap(err, "failed to submit retina-hubble to workerpool")
				}
				return nil
			},
			OnStop: func(cell.HookContext) error {
				if err := wp.Close(); err != nil {
					return errors.Wrap(err, "failed to close retina-hubble workerpool")
				}
				return nil
			},
		})
	}),
)
