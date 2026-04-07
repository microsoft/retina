// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

// Package summary collects per-workflow and per-scenario results and renders a
// Markdown table suitable for GitHub Actions step summaries.
package summary

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Status represents the outcome of a workflow or scenario.
type Status string

const (
	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// WorkflowResult records the outcome of a top-level workflow.
type WorkflowResult struct {
	Name     string
	Status   Status
	Duration time.Duration
	Err      string
}

// SkippedScenario records a scenario that was intentionally skipped.
type SkippedScenario struct {
	Workflow string
	Scenario string
	Reason   string
}

// TestSummary accumulates results across all workflows and scenarios.
type TestSummary struct {
	mu        sync.Mutex
	workflows []WorkflowResult
	skipped   []SkippedScenario
	provider  string
}

// New creates a TestSummary for the given provider (e.g. "kind", "azure").
func New(provider string) *TestSummary {
	return &TestSummary{provider: provider}
}

// AddWorkflow records the result of a top-level workflow.
func (s *TestSummary) AddWorkflow(name string, status Status, dur time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := WorkflowResult{Name: name, Status: status, Duration: dur}
	if err != nil {
		r.Err = err.Error()
	}
	s.workflows = append(s.workflows, r)
}

// Skip records a scenario that was intentionally skipped.
func (s *TestSummary) Skip(workflow, scenario, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipped = append(s.skipped, SkippedScenario{
		Workflow: workflow,
		Scenario: scenario,
		Reason:   reason,
	})
}

func statusIcon(st Status) string {
	switch st {
	case Passed:
		return "✅"
	case Failed:
		return "❌"
	case Skipped:
		return "⏭️"
	default:
		return "❓"
	}
}

// WriteMarkdown renders a Markdown summary to w.
func (s *TestSummary) WriteMarkdown(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var b strings.Builder

	b.WriteString("## E2E Test Summary\n\n")
	b.WriteString(fmt.Sprintf("**Provider:** `%s`\n\n", s.provider))

	// Workflow results table.
	b.WriteString("### Workflow Results\n\n")
	b.WriteString("| Workflow | Status | Duration | Details |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range s.workflows {
		detail := ""
		if r.Err != "" {
			// Truncate long errors for the table.
			detail = r.Err
			if len(detail) > 120 {
				detail = detail[:120] + "…"
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s %s | %s | %s |\n",
			r.Name, statusIcon(r.Status), r.Status, r.Duration.Round(time.Second), detail))
	}

	// Skipped scenarios table.
	if len(s.skipped) > 0 {
		b.WriteString("\n### Skipped Scenarios\n\n")
		b.WriteString("| Workflow | Scenario | Reason |\n")
		b.WriteString("|---|---|---|\n")
		for _, sk := range s.skipped {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", sk.Workflow, sk.Scenario, sk.Reason))
		}
	}

	// Overall counts.
	passed, failed, skippedWf := 0, 0, 0
	for _, r := range s.workflows {
		switch r.Status {
		case Passed:
			passed++
		case Failed:
			failed++
		case Skipped:
			skippedWf++
		}
	}
	b.WriteString(fmt.Sprintf("\n**Total:** %d passed, %d failed, %d skipped workflows | %d skipped scenarios\n",
		passed, failed, skippedWf, len(s.skipped)))

	_, err := io.WriteString(w, b.String())
	return err
}
