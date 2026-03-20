//go:build e2e

// Package retina contains the e2e test entry point.
//
// A single test function drives three phases — image build, infrastructure
// provisioning, and workflow tests — so that `go test -tags=e2e -provider=kind`
// is all you need for a full local run.
package retina

import (
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	"github.com/microsoft/retina/test/e2ev3/pkg/images/build"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra"
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
	ctx, cancel := config.TestContext(t)
	defer cancel()

	p := &config.E2EConfig{}

	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Pipe(
		&config.Step{Cfg: p},
		&build.Step{Cfg: p},
		&infra.Workflow{Cfg: p, T: t},
		&images.Step{Cfg: p},
		&basicmetrics.Workflow{Cfg: p},
		&advancedmetrics.Workflow{Cfg: p},
		&hubblemetrics.Workflow{Cfg: p},
		&basicexp.Workflow{Cfg: p},
		&advexp.Workflow{Cfg: p},
		&capture.Workflow{Cfg: p},
	))
	require.NoError(t, wf.Do(ctx), "e2e workflow failed")
}
