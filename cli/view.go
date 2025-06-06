package main

import (
	"fmt"
	"os"
	"os/exec"
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
	confirmed              bool
	selectedType           string
	selectedNamespace      string
	selectedLabel          string
	selectedNamespaceLabel string
	nsSelectorForView      string
	step                   selectStep
	labelOptions           []string
	labelToNS              map[string][]string
	labelToName            map[string][]string
}

// Helper to create a table
func newTable(cols []table.Column, rows []table.Row, prompt string, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
	t.SetHeight(height)
	return t
}

// Initial model
func initialModel() model {
	cols := []table.Column{{Title: "Type", Width: 15}}
	rows := []table.Row{{"Pod"}, {"Deployment"}, {"DaemonSet"}}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	t := newTable(cols, rows, "Select a resource type:", 5)
	return model{table: t, prompt: "Select a resource type:", step: stepType}
}

// Helper to run a command in the terminal asynchronously
func runCommandInTerminal(cmdStr string) error {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Print a separator before running the command for better output clarity
	fmt.Println("\n================ Retina CLI Output ================\n")
	// Set environment to preserve terminal formatting (including newlines/tabs)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	err := cmd.Run()
	fmt.Println("\n================ End Retina CLI Output ================\n")
	return err
}

// Add back toMM and fromMM helpers for model <-> MainModel conversion
func (m *model) toMM() captureviews.MainModel {
	return captureviews.MainModel{
		Table:                  m.table,
		Prompt:                 m.prompt,
		Confirmed:              m.confirmed,
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
	m.confirmed = mm.Confirmed
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
			return m, nil
		case "y":
			if m.confirmed && m.step == stepDone {
				var nsSelector string
				nsSelector = m.selectedNamespaceLabel
				hostPath := "/mnt/retina/captures"
				cmd := fmt.Sprintf(
					"kubectl retina capture create --name retina-capture --host-path %s --namespace-selectors \"%s\" --pod-selectors \"%s\"",
					hostPath,
					nsSelector,
					m.selectedLabel,
				)
				// Run the command in the terminal
				go func() {
					_ = runCommandInTerminal(cmd)
				}()
				m.prompt = "Command is being executed in the terminal."
				return m, nil
			}
		case "n":
			if m.confirmed && m.step == stepDone {
				m.prompt = "Command not executed."
				return m, nil
			}
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
	if mFinal.confirmed && mFinal.step == stepDone {
		nsSelector := mFinal.selectedNamespaceLabel
		hostPath := "/mnt/retina/captures"
		// Build args for cobra: retina capture create --name retina-capture --host-path ... --namespace-selectors ... --pod-selectors ...
		args := []string{
			"capture", "create",
			"--name", "retina-capture",
			"--host-path", hostPath,
			"--namespace-selectors", nsSelector,
			"--pod-selectors", mFinal.selectedLabel,
		}
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
	if m.confirmed && m.step == stepDone {
		var nsSelector string
		nsSelector = m.selectedNamespaceLabel
		hostPath := "/mnt/retina/captures"
		cmd := fmt.Sprintf(
			"kubectl retina capture create --name retina-capture --host-path %s --namespace-selectors \"%s\" --pod-selectors \"%s\"",
			hostPath,
			nsSelector,
			m.selectedLabel, // always use the selected label as the pod selector
		)
		return fmt.Sprintf(
			"Type: %s\nNamespace: %s\nNamespace selector: %s\nLabel selector: %s\n\nRetina CLI command:\n%s\n\nWould you like to run this command? (y/n)",
			m.selectedType, m.selectedNamespace, nsSelector, m.selectedLabel, cmd,
		)
	}
	return fmt.Sprintf("%s\n\n%s\n\n(Use ↑/↓ to move, enter to select, q to quit)",
		m.prompt,
		m.table.View(),
	)
}
