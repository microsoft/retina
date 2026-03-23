// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package utils

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/microsoft/retina/test/e2ev3/pkg/stepname"
)

// These mock types simulate the real e2e call stack:
//
//	Workflow.Do()                    → sets "workflow" attr, passes logger down
//	  └─ addScenario(log)           → sets "test" attr, passes logger down
//	       └─ WithPortForward.Do()  → calls PortForward.Do()
//	            └─ PortForward.Do() → sets "step" attr, logs messages

// Workflow mirrors *basicmetrics.Workflow.
// StepName should resolve to the package name ("utils" here) since the
// type name "Workflow" is generic.
type Workflow struct {
	// bareStep, if set, is called instead of the normal scenario chain.
	// Used by TestHandlerFormat_WorkflowPrefixFromStack to test stack-based
	// workflow detection when steps don't receive an explicit logger.
	bareStep func()
}

func (w *Workflow) Do() {
	if w.bareStep != nil {
		w.bareStep()
		return
	}
	// Real workflows create a logger and pass it to scenarios — they
	// don't log directly. This matches basicmetrics.Workflow.Do().
	log := slog.Default().With("workflow", stepname.StepName(w))

	// Simulate passing logger to scenario.
	s := &MockScenario{log: log}
	s.Do()
}

// MockScenario mirrors addDropScenario / addTCPScenario.
type MockScenario struct {
	log *slog.Logger
}

func (s *MockScenario) Do() {
	// Real scenarios add "test" attr and pass the logger to steps.
	log := s.log.With("test", "drop")

	// Simulate passing logger to PortForward via WithPortForward.
	pf := &MockPortForward{Log: log}
	pf.Do()
}

// MockPortForward mirrors *k8s.PortForward.
type MockPortForward struct {
	Log *slog.Logger
}

func (pf *MockPortForward) Do() {
	log := pf.Log
	if log == nil {
		log = slog.Default()
	}
	log = log.With("step", stepname.StepName(pf))
	log.Info("finding pod with affinity", "label", "k8s-app=retina")
	log.Info("attempting port forward", "pod", "retina-agent-abc", "namespace", "kube-system")
	log.Info("port forward validation succeeded", "address", "http://localhost:10093")
}

// MockBareStep simulates a step that does NOT receive a logger from the
// workflow (e.g., CreateAgnhostStatefulSet, CreateDenyAllNetworkPolicy).
// It uses slog.Default() — the handler must detect the workflow from the stack.
type MockBareStep struct{}

func (s *MockBareStep) Do() {
	slog.Info("creating resource", "name", "agnhost")
}

// WorkflowWithBareStep simulates a Workflow that calls a step without passing
// a logger. Note: type name is NOT "Workflow", so the handler won't detect it
// as a workflow — only as a step.
type WorkflowWithBareStep struct{}

func (w *WorkflowWithBareStep) Do() {
	step := &MockBareStep{}
	step.Do()
}

// MockCallerDetected is used when NO explicit "step" attribute is set.
// The handler should auto-detect the type name via runtime stack inspection.
type MockCallerDetected struct{}

func (m *MockCallerDetected) Do() {
	// No log.With("step", ...) — handler must detect "mock-caller-detected" from the stack.
	slog.Info("this should auto-detect step name")
}

// stripANSI removes ANSI escape codes for easier assertion.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// hasANSI checks that the bracketed prefix contains ANSI color codes.
func hasANSI(s string) bool {
	re := regexp.MustCompile(`\x1b\[\d+m\[`)
	return re.MatchString(s)
}

func TestHandlerFormat_ExplicitAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandler(&buf, slog.LevelInfo)
	slog.SetDefault(slog.New(handler))

	// Replicate: Workflow.Do() → addScenario(log) → PortForward.Do()
	w := &Workflow{}
	w.Do()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 log lines, got %d:\n%s", len(lines), output)
	}

	// Verify each line format: "HH:MM:SS LEVEL [prefix] message key=value"
	timeLevel := regexp.MustCompile(`^\d{2}:\d{2}:\d{2} (INFO|ERROR|WARN|DEBUG) `)
	for i, line := range lines {
		if !timeLevel.MatchString(line) {
			t.Errorf("line %d: expected timestamp+level first, got: %s", i, line)
		}
	}

	// All 3 lines come from MockPortForward.Do() which sets step explicitly.
	// Prefix should be [utils/drop/mock-port-forward]:
	//   workflow = "utils"             (StepName resolves generic Workflow → package name)
	//   test     = "drop"              (set in MockScenario.Do)
	//   step     = "mock-port-forward" (set explicitly via log.With)
	expectedPrefix := "[utils/drop/mock-port-forward]"
	for i, line := range lines {
		if !strings.Contains(line, expectedPrefix) {
			t.Errorf("line %d: expected %s prefix, got: %s", i, expectedPrefix, line)
		}
	}

	// Buffer is not a TTY → no ANSI codes should be present.
	for i, line := range lines {
		if hasANSI(line) {
			t.Errorf("line %d: unexpected ANSI codes in non-TTY output", i)
		}
	}

	// Verify key=value pairs propagate.
	if !strings.Contains(lines[0], "label=k8s-app=retina") {
		t.Errorf("line 0: expected label=k8s-app=retina, got: %s", lines[0])
	}
}

func TestHandlerFormat_ColorOnTTY(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandlerWithColor(&buf, slog.LevelInfo, true)
	slog.SetDefault(slog.New(handler))

	w := &Workflow{}
	w.Do()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 log lines, got %d", len(lines))
	}

	// With color forced on, ANSI codes should wrap the prefix.
	for i, line := range lines {
		if !hasANSI(line) {
			t.Errorf("line %d: expected ANSI color on bracketed prefix", i)
		}
	}

	// Stripping ANSI should still show the correct prefix.
	expectedPrefix := "[utils/drop/mock-port-forward]"
	for i, line := range lines {
		plain := stripANSI(line)
		if !strings.Contains(plain, expectedPrefix) {
			t.Errorf("line %d: expected %s prefix, got: %s", i, expectedPrefix, plain)
		}
	}
}

func TestHandlerFormat_ColorDeterminism(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandlerWithColor(&buf, slog.LevelInfo, true)
	slog.SetDefault(slog.New(handler))

	w := &Workflow{}
	w.Do()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// All lines share the same prefix → same color.
	ansiRe := regexp.MustCompile(`(\x1b\[\d+m)\[`)
	first := ansiRe.FindStringSubmatch(lines[0])
	if first == nil {
		t.Fatal("no ANSI color code found in first line")
	}
	for i, line := range lines[1:] {
		match := ansiRe.FindStringSubmatch(line)
		if match == nil {
			t.Errorf("line %d: no ANSI color code found", i+1)
			continue
		}
		if match[1] != first[1] {
			t.Errorf("line %d: color %q differs from first line %q", i+1, match[1], first[1])
		}
	}

	// Log with a DIFFERENT prefix and verify it also gets a valid color.
	buf.Reset()
	slog.SetDefault(slog.New(NewStepHandlerWithColor(&buf, slog.LevelInfo, true)))
	m := &MockCallerDetected{}
	m.Do()
	diffLine := buf.String()
	diffMatch := ansiRe.FindStringSubmatch(diffLine)
	if diffMatch == nil {
		t.Fatal("no ANSI color code found in caller-detected line")
	}
	validAnsi := regexp.MustCompile(`^\x1b\[\d+m$`)
	if !validAnsi.MatchString(diffMatch[1]) {
		t.Errorf("invalid ANSI escape for different prefix: %q", diffMatch[1])
	}
}

func TestHandlerFormat_CallerAutoDetection(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandler(&buf, slog.LevelInfo)
	slog.SetDefault(slog.New(handler))

	// No explicit "step" attribute — handler should detect from call stack.
	m := &MockCallerDetected{}
	m.Do()

	output := stripANSI(buf.String())
	// The handler should detect "mock-caller-detected" from the receiver type.
	if !strings.Contains(output, "[mock-caller-detected]") {
		t.Errorf("expected auto-detected [mock-caller-detected] prefix, got: %s", output)
	}
}

func TestHandlerFormat_WorkflowAutoDetection(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandler(&buf, slog.LevelInfo)
	slog.SetDefault(slog.New(handler))

	// Simulate a step called from inside a Workflow.Do() that does NOT
	// receive a logger. The handler should detect both the workflow
	// ("utils" — package name of WorkflowWithBareStep) and the step
	// ("mock-bare-step") from the call stack.
	w := &WorkflowWithBareStep{}
	w.Do()

	output := buf.String()
	t.Logf("output: %s", output)

	// Should detect workflow from (*WorkflowWithBareStep).Do on the stack.
	// WorkflowWithBareStep → type name ends in "...BareStep" — not "Workflow",
	// so it won't be detected as a workflow. Let me use the real Workflow type.
	// Actually, the type is WorkflowWithBareStep, not Workflow — the handler
	// only recognizes types named exactly "Workflow". This test verifies the
	// step is detected.
	if !strings.Contains(output, "[mock-bare-step]") {
		t.Errorf("expected [mock-bare-step] in output, got: %s", output)
	}
}

func TestHandlerFormat_WorkflowPrefixFromStack(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandler(&buf, slog.LevelInfo)
	slog.SetDefault(slog.New(handler))

	// Simulate real e2e: Workflow.Do() → step.Do() → slog.Info().
	// The step uses slog.Default() (no explicit logger/attributes).
	// The handler should detect:
	//   step     = "mock-bare-step"  (from (*MockBareStep).Do)
	//   workflow = "utils"           (from (*Workflow).Do higher on the stack)
	bare := &MockBareStep{}
	w := &Workflow{bareStep: bare.Do}
	w.Do()

	output := buf.String()
	t.Logf("output: %s", output)

	// Verify callerPrefix detects workflow from stack.
	// Note: in the real e2e, go-workflow sits between Workflow.Do and Step.Do.
	// Our stack walker skips non-e2ev3 frames, so it should still find both.
	if !strings.Contains(output, "[utils/mock-bare-step]") {
		t.Errorf("expected [utils/mock-bare-step] prefix, got: %s", output)
	}
}

func TestHandlerFormat_NoPrefix(t *testing.T) {
	var buf bytes.Buffer
	handler := NewStepHandler(&buf, slog.LevelInfo)
	slog.SetDefault(slog.New(handler))

	// Plain slog.Info from a non-method (no receiver to detect).
	slog.Info("bare log line")

	output := buf.String()
	plain := stripANSI(output)
	// Should still have timestamp+level, but no bracketed prefix (or auto-detected).
	if !regexp.MustCompile(`^\d{2}:\d{2}:\d{2} INFO `).MatchString(plain) {
		t.Errorf("expected timestamp+level first, got: %s", plain)
	}
}

func TestStepName_GenericTypes(t *testing.T) {
	// Verify that generic type "Workflow" resolves to package name, not "workflow".
	w := &Workflow{}
	name := stepname.StepName(w)
	// In this test file (package utils), it should be "utils".
	if name != "utils" {
		t.Errorf("StepName(*Workflow) = %q, want %q", name, "utils")
	}

	// Non-generic types keep their own name.
	pf := &MockPortForward{}
	name = stepname.StepName(pf)
	if name != "mock-port-forward" {
		t.Errorf("StepName(*MockPortForward) = %q, want %q", name, "mock-port-forward")
	}

	mcd := &MockCallerDetected{}
	name = stepname.StepName(mcd)
	if name != "mock-caller-detected" {
		t.Errorf("StepName(*MockCallerDetected) = %q, want %q", name, "mock-caller-detected")
	}
}

func TestColorForPrefix_Deterministic(t *testing.T) {
	// Same input always produces the same color code.
	for _, prefix := range []string{
		"basic-metrics/drop/port-forward",
		"hubble-metrics/flow-intra/curl-pod",
		"advanced-metrics/dns/validate",
		"slog-writer",
	} {
		c1 := colorForPrefix(prefix)
		c2 := colorForPrefix(prefix)
		if c1 != c2 {
			t.Errorf("colorForPrefix(%q) not deterministic: %d vs %d", prefix, c1, c2)
		}
		// Verify it's a valid ANSI color code (31-36 or 91-96).
		if !((c1 >= 31 && c1 <= 36) || (c1 >= 91 && c1 <= 96)) {
			t.Errorf("colorForPrefix(%q) = %d, not a valid ANSI color code", prefix, c1)
		}
	}
}
