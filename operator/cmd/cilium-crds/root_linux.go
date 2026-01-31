// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

// NOTE: this file was originally a modified/slimmed-down version of Cilium's operator
// to provide Retina with a hive to run Cilium's garbage collection Cells.
// Now, it contains Retina-related code ported into Cells.

package ciliumcrds

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	operatorOption "github.com/cilium/cilium/operator/option"
	"github.com/cilium/cilium/pkg/hive"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	k8sversion "github.com/cilium/cilium/pkg/k8s/version"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var (
	// set logger field: subsys=retina-operator
	binaryName       = filepath.Base(os.Args[0])
	slogLogger       = logging.DefaultSlogLogger.With(logfields.LogSubsys, binaryName)
	operatorIDLength = 10
)

func Execute(h *hive.Hive) {
	initEnv(h.Viper())

	// Use zap-backed slog logger for hive (routes to stdout + Application Insights)
	if err := h.Run(log.SlogLogger()); err != nil {
		logging.Fatal(slogLogger, err.Error())
	}
}

func registerOperatorHooks(
	l *slog.Logger, lc cell.Lifecycle, llc *LeaderLifecycle,
	clientset k8sClient.Clientset, shutdowner hive.Shutdowner,
) {
	var wg sync.WaitGroup
	lc.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			wg.Add(1)
			go func() {
				runOperator(l, llc, clientset, shutdowner)
				wg.Done()
			}()
			return nil
		},
		OnStop: func(ctx cell.HookContext) error {
			if err := llc.Stop(logging.DefaultSlogLogger, ctx); err != nil {
				return errors.Wrap(err, "failed to stop operator")
			}
			doCleanup()
			wg.Wait()
			return nil
		},
	})
}

func initEnv(vp *viper.Viper) {
	// Prepopulate option.Config with options from CLI.

	// NOTE: if the flag is not provided in operator/cmd/flags.go InitGlobalFlags(), these Populate methods override
	// the default values provided in option.Config or operatorOption.Config respectively.
	// The values will be overridden to the "zero value".
	// Maybe could create a cell.Config for these instead?
	// slogloggercheck: using default logger for configuration initialization
	option.Config.Populate(logging.DefaultSlogLogger, vp)
	operatorOption.Config.Populate(logging.DefaultSlogLogger, vp)

	// add hooks after setting up metrics in the option.Config
	logging.AddHandlers(metrics.NewLoggingHook())

	// Logging should always be bootstrapped first. Do not add any code above this!
	if err := logging.SetupLogging(option.Config.LogDriver, logging.LogOptions(option.Config.LogOpt), binaryName, option.Config.Debug); err != nil {
		logging.Fatal(slogLogger, err.Error())
	}

	// Set up zap logger with Application Insights telemetry
	_, _ = log.SetupZapLogger(&log.LogOpts{
		Level:                 option.Config.LogOpt[logging.LevelOpt],
		ApplicationInsightsID: buildinfo.ApplicationInsightsID,
		EnableTelemetry:       buildinfo.ApplicationInsightsID != "",
	}, zap.String("version", buildinfo.Version))
	log.SetDefaultSlog()

	option.LogRegisteredSlogOptions(vp, slogLogger)
	slogLogger.Info("retina operator version", "version", buildinfo.Version)
}

func doCleanup() {
	isLeader.Store(false)

	// Cancelling this context here makes sure that if the operator hold the
	// leader lease, it will be released.
	leaderElectionCtxCancel()
}

// runOperator implements the logic of leader election for cilium-operator using
// built-in leader election capability in kubernetes.
// See: https://github.com/kubernetes/client-go/blob/master/examples/leader-election/main.go
func runOperator(l *slog.Logger, lc *LeaderLifecycle, clientset k8sClient.Clientset, shutdowner hive.Shutdowner) {
	isLeader.Store(false)

	leaderElectionCtx, leaderElectionCtxCancel = context.WithCancel(context.Background())

	// We only support Operator in HA mode for Kubernetes Versions having support for
	// LeasesResourceLock.
	// See docs on capabilities.LeasesResourceLock for more context.
	if !k8sversion.Capabilities().LeasesResourceLock {
		l.Info("Support for coordination.k8s.io/v1 not present, fallback to non HA mode")

		if err := lc.Start(logging.DefaultSlogLogger, leaderElectionCtx); err != nil {
			logging.Fatal(l, "Failed to start leading", logfields.Error, err)
		}
		return
	}

	// Get hostname for identity name of the lease lock holder.
	// We identify the leader of the operator cluster using hostname.
	operatorID, err := os.Hostname()
	if err != nil {
		logging.Fatal(l, "Failed to get hostname when generating lease lock identity", logfields.Error, err)
	}
	operatorID, err = randomStringWithPrefix(operatorID+"-", operatorIDLength)
	if err != nil {
		logging.Fatal(l, "Failed to generate random string for lease lock identity", logfields.Error, err)
	}

	leResourceLock, err := resourcelock.NewFromKubeconfig(
		resourcelock.LeasesResourceLock,
		operatorK8sNamespace,
		leaderElectionResourceLockName,
		resourcelock.ResourceLockConfig{
			// Identity name of the lock holder
			Identity: operatorID,
		},
		clientset.RestConfig(),
		operatorOption.Config.LeaderElectionRenewDeadline)
	if err != nil {
		logging.Fatal(l, "Failed to create resource lock for leader election", logfields.Error, err)
	}

	// Start the leader election for running cilium-operators
	l.Info("Waiting for leader election")
	leaderelection.RunOrDie(leaderElectionCtx, leaderelection.LeaderElectionConfig{
		Name: leaderElectionResourceLockName,

		Lock:            leResourceLock,
		ReleaseOnCancel: true,

		LeaseDuration: operatorOption.Config.LeaderElectionLeaseDuration,
		RenewDeadline: operatorOption.Config.LeaderElectionRenewDeadline,
		RetryPeriod:   operatorOption.Config.LeaderElectionRetryPeriod,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				if err := lc.Start(logging.DefaultSlogLogger, ctx); err != nil {
					l.Error("Failed to start when elected leader, shutting down", logfields.Error, err)
					shutdowner.Shutdown(hive.ShutdownWithError(err))
				}
			},
			OnStoppedLeading: func() {
				l.Info("Leader election lost", "operator-id", operatorID)
				// Cleanup everything here, and exit.
				shutdowner.Shutdown(hive.ShutdownWithError(errors.New("Leader election lost")))
			},
			OnNewLeader: func(identity string) {
				if identity == operatorID {
					l.Info("Leading the operator HA deployment")
				} else {
					l.Info("Leader re-election complete", "newLeader", identity, "operatorID", operatorID)
				}
			},
		},
	})
}

// RandomStringWithPrefix returns a random string of length n + len(prefix) with
// the given prefix, containing upper- and lowercase runes.
func randomStringWithPrefix(prefix string, n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, n)
	for i := range bytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		bytes[i] = letters[num.Int64()]
	}
	return prefix + string(bytes), nil
}
