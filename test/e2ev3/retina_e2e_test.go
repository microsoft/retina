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
	"github.com/microsoft/retina/test/e2ev3/pkg/summary"
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
	sum := summary.New(*config.Provider)
	c.Summary = sum

	// Write the test summary at the end of the run, regardless of outcome.
	// If $GITHUB_STEP_SUMMARY is set, append there; always write to test-summary.md.
	t.Cleanup(func() { writeSummary(t, sum) })

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

	// Wrap each workflow so its duration and pass/fail are recorded.
	track := func(s flow.Steper) flow.Steper {
		return &summary.TrackedWorkflow{Inner: s, Summary: sum}
	}
	pipeline := []flow.Steper{
		track(basic), track(advanced), track(hubble),
		track(basicExp), track(advExp), track(cap),
	}

	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.BatchPipe(
		flow.Steps(loadConfig).Timeout(1*time.Minute),
		flow.Steps(buildImages, setupInfra).Timeout(30*time.Minute),
		flow.Steps(loadImages).Timeout(10*time.Minute),
		flow.Pipe(pipeline...),
	))

	require.NoError(t, wf.Do(ctx), "e2e workflow failed")
}

// writeSummary renders the test summary to a local file and, when running in
// GitHub Actions, appends it to $GITHUB_STEP_SUMMARY.
func writeSummary(t *testing.T, sum *summary.TestSummary) {
	t.Helper()

	// Always write a local file.
	const localPath = "test-summary.md"
	f, err := os.Create(localPath)
	if err != nil {
		t.Logf("warning: could not create %s: %v", localPath, err)
		return
	}
	defer f.Close()
	if err := sum.WriteMarkdown(f); err != nil {
		t.Logf("warning: could not write summary: %v", err)
	}
	t.Logf("test summary written to %s", localPath)

	// Append to GitHub Actions step summary if available.
	ghPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if ghPath == "" {
		return
	}
	gh, err := os.OpenFile(ghPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Logf("warning: could not open $GITHUB_STEP_SUMMARY: %v", err)
		return
	}
	defer gh.Close()
	if err := sum.WriteMarkdown(gh); err != nil {
		t.Logf("warning: could not write step summary: %v", err)
	}
	t.Logf("test summary appended to $GITHUB_STEP_SUMMARY")
}
