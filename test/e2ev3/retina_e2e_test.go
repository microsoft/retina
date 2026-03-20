//go:build e2e

// Package retina contains the e2e test entry point.
//
// A single test function drives three phases — image build, infrastructure
// provisioning, and workflow tests — so that `go test -tags=e2e -provider=kind`
// is all you need for a full local run.
package retina

import (
	"log/slog"
	"os"
	"testing"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	"github.com/microsoft/retina/test/e2ev3/pkg/images/build"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"github.com/microsoft/retina/test/e2ev3/workflows/advancedmetrics"
	advexp "github.com/microsoft/retina/test/e2ev3/workflows/advancedmetrics/experimental"
	"github.com/microsoft/retina/test/e2ev3/workflows/basicmetrics"
	basicexp "github.com/microsoft/retina/test/e2ev3/workflows/basicmetrics/experimental"
	"github.com/microsoft/retina/test/e2ev3/workflows/capture"
	"github.com/microsoft/retina/test/e2ev3/workflows/hubblemetrics"
	"github.com/stretchr/testify/require"
)

// TestE2ERetina drives image build, cluster provisioning, and all Retina
// workflow tests in sequence against a single cluster.
func TestE2ERetina(t *testing.T) {
	slog.SetDefault(slog.New(utils.NewStepHandler(os.Stderr, slog.LevelInfo)))

	ctx, cancel := config.TestContext(t)
	defer cancel()

	c := &config.E2EConfig{}

	loadConfig := &config.Step{Cfg: c}
	buildImages := &build.Step{Cfg: c}
	setupInfra := &infra.Workflow{Cfg: c, T: t}
	loadImages := &images.Step{Cfg: c}

	basic := &basicmetrics.Workflow{Cfg: c}
	advanced := &advancedmetrics.Workflow{Cfg: c}
	hubble := &hubblemetrics.Workflow{Cfg: c}
	basicExp := &basicexp.Workflow{Cfg: c}
	advExp := &advexp.Workflow{Cfg: c}
	cap := &capture.Workflow{Cfg: c}

	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.BatchPipe(
		flow.Steps(loadConfig).Timeout(1*time.Minute),
		flow.Steps(buildImages, setupInfra).Timeout(30*time.Minute),
		flow.Steps(loadImages).Timeout(10*time.Minute),
		flow.Pipe(basic, advanced, hubble, basicExp, advExp, cap),
	))

	require.NoError(t, wf.Do(ctx), "e2e workflow failed")
}
