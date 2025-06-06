package captureviews

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/microsoft/retina/cli/internal/kubehelpers"
)

type MainModel struct {
	Table                  table.Model
	Prompt                 string
	Confirmed              bool
	SelectedType           string
	SelectedNamespace      string
	SelectedLabel          string
	SelectedNamespaceLabel string
	NsSelectorForView      string
	Step                   int
	LabelOptions           []string
	LabelToNS              map[string][]string
	LabelToName            map[string][]string
}

type ResourceRow struct {
	NamespaceNames string
	NamespaceLabel string
}

type LabelRow struct {
	ResourceNames string
	LabelSelector string
}

func ToResourceRow(row table.Row) ResourceRow {
	return ResourceRow{
		NamespaceNames: getStringAt(row, 0),
		NamespaceLabel: getStringAt(row, 1),
	}
}

func ToLabelRow(row table.Row) LabelRow {
	return LabelRow{
		ResourceNames: getStringAt(row, 0),
		LabelSelector: getStringAt(row, 1),
	}
}

func getStringAt(row table.Row, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func NewTable(cols []table.Column, rows []table.Row, prompt string, height int) table.Model {
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
	t.SetHeight(height)
	return t
}

func JoinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// Real implementations for external dependencies
func GetNamespaceLabels() ([]string, map[string][]string, error) {
	return kubehelpers.GetNamespaceLabels()
}
func GetResourceLabels(resourceType, ns string) ([]string, map[string][]string, error) {
	return kubehelpers.GetResourceLabels(resourceType, ns)
}

const (
	StepType = iota
	StepNamespace
	StepLabel
	StepDone
)
