package main

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	captureviews "github.com/microsoft/retina/cli/views/capture"
)

// Model step constants
const (
	stepType selectStep = iota
	stepNamespace
	stepLabel
	stepDone
)

type selectStep int

type model struct {
	table                  table.Model
	prompt                 string
	selectedType           string
	selectedNamespace      string
	selectedLabel          string
	selectedNamespaceLabel string
	nsSelectorForView      string
	step                   selectStep
	labelOptions           []string
	labelToNS              map[string][]string
	labelToName            map[string][]string
	done                   bool // track if final state is reached
}

// Helper to create a table
func newTable(cols []table.Column, rows []table.Row, prompt string, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
	t.SetHeight(height)
	return t
}

// Initial model
func initialModel() model {
	// Get namespace label selectors for the initial prompt
	labels, labelToNS, err := captureviews.GetNamespaceLabels()
	if err != nil || len(labels) == 0 {
		labels = []string{"none found"}
		labelToNS = map[string][]string{"none found": {}}
	}
	rows := make([]table.Row, 0, len(labels))
	for _, label := range labels {
		if nsList, ok := labelToNS[label]; ok && len(nsList) > 0 {
			rows = append(rows, table.Row{captureviews.JoinOrNone(nsList), label})
		}
	}
	if len(rows) == 0 {
		rows = append(rows, table.Row{"none found", "none found"})
	}
	// Sort rows by the left column (Namespace(s))
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})
	cols := []table.Column{{Title: "Namespace(s)", Width: 80}, {Title: "Namespace Label Selector", Width: 40}}
	t := newTable(cols, rows, "Select a namespace label selector:", 15)
	return model{
		table:        t,
		prompt:       "Select a namespace label selector:",
		step:         stepNamespace,
		labelOptions: labels,
		labelToNS:    labelToNS,
	}
}

// Add back toMM and fromMM helpers for model <-> MainModel conversion
func (m *model) toMM() captureviews.MainModel {
	return captureviews.MainModel{
		Table:                  m.table,
		Prompt:                 m.prompt,
		SelectedType:           m.selectedType,
		SelectedNamespace:      m.selectedNamespace,
		SelectedLabel:          m.selectedLabel,
		SelectedNamespaceLabel: m.selectedNamespaceLabel,
		NsSelectorForView:      m.nsSelectorForView,
		Step:                   int(m.step),
		LabelOptions:           m.labelOptions,
		LabelToNS:              m.labelToNS,
		LabelToName:            m.labelToName,
	}
}

func (m *model) fromMM(mm captureviews.MainModel) {
	m.table = mm.Table
	m.prompt = mm.Prompt
	m.selectedType = mm.SelectedType
	m.selectedNamespace = mm.SelectedNamespace
	m.selectedLabel = mm.SelectedLabel
	m.selectedNamespaceLabel = mm.SelectedNamespaceLabel
	m.nsSelectorForView = mm.NsSelectorForView
	m.step = selectStep(mm.Step)
	m.labelOptions = mm.LabelOptions
	m.labelToNS = mm.LabelToNS
	m.labelToName = mm.LabelToName
}

// Main update logic, simplified
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Push current state to the stack before progressing
			modelStack = append(modelStack, m)
			mm := m.toMM()
			switch m.step {
			case stepType:
				captureviews.HandleStepKubernetesResourceType(&mm)
			case stepNamespace:
				captureviews.HandleStepNamespace(&mm)
			case stepLabel:
				captureviews.HandleStepLabel(&mm)
			}
			m.fromMM(mm)
			if m.step == stepDone {
				m.done = true
				return m, tea.Quit
			}
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			return goBackStack(), nil
		}
	}
	m.table, _ = m.table.Update(msg)
	return m, nil
}

// Add a global stack to persist model states for back navigation
var modelStack []model

// Go back one step in the TUI flow using the global stack
func goBackStack() model {
	if len(modelStack) > 1 {
		// Pop the current state
		modelStack = modelStack[:len(modelStack)-1]
		// Return the previous state
		return modelStack[len(modelStack)-1]
	} else if len(modelStack) == 1 {
		// Only the initial state remains
		return modelStack[0]
	}
	// If stack is empty, return a fresh initial model
	return initialModel()
}

// Helper to build the CLI args for retina cobra
func buildArgs(ns, podSelector, nsSelector string) []string {
	if ns == "" {
		ns = "default"
	}
	return []string{
		"capture", "create",
		"--namespace", ns,
		fmt.Sprintf("--pod-selectors=%q", podSelector),
		fmt.Sprintf("--namespace-selectors=%q", nsSelector),
	}
}

// RunTUI launches the TUI and returns the CLI args if the user confirms, or nil if cancelled.
func RunTUI() ([]string, error) {
	m := initialModel()
	modelStack = []model{m}
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	mFinal, ok := finalModel.(model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	if mFinal.step == stepDone {
		args := buildArgs(mFinal.selectedNamespace, mFinal.selectedLabel, mFinal.selectedNamespaceLabel)
		return args, nil
	}
	return nil, nil // user cancelled or did not confirm
}

// Step handler for stepType
// moved to handler_step_type.go

// Step handler for stepNamespace
// moved to handler_step_namespace.go

// Step handler for stepLabel
// moved to handler_step_label.go

// Ensure model implements tea.Model
func (m model) Init() tea.Cmd { return nil }

// View logic remains unchanged
func (m model) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s\n\n%s\n\n(Use ↑/↓ to move, enter to select, q to quit)",
		m.prompt,
		m.table.View(),
	)
}

func needsQuoting(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' {
			return true
		}
	}
	return false
}
