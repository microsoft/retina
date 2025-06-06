package captureviews

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
)

// Exported handler for namespace step
func HandleStepNamespace(m *MainModel) {
	selected := m.Table.SelectedRow()
	row := ToResourceRow(selected)
	m.Prompt = "Select a resource label selector:"
	m.SelectedNamespaceLabel = row.NamespaceLabel
	m.SelectedLabel = row.NamespaceLabel
	m.NsSelectorForView = row.NamespaceLabel
	m.SelectedNamespace = ""
	if nsList, ok := m.LabelToNS[row.NamespaceLabel]; ok {
		for _, ns := range nsList {
			if ns != "none found" && ns != "none" && ns != "" {
				m.SelectedNamespace = ns
				break
			}
		}
	}
	if m.SelectedNamespace == "" && row.NamespaceNames != "none found" && row.NamespaceNames != "" {
		names := strings.Split(row.NamespaceNames, ", ")
		for _, name := range names {
			if name != "none" && name != "" {
				m.SelectedNamespace = name
				break
			}
		}
	}
	labels, labelToName, err := GetResourceLabels(strings.ToLower(m.SelectedType), m.SelectedNamespace)
	if err != nil || len(labels) == 0 {
		labels = []string{"none found"}
		labelToName = map[string][]string{"none found": {}}
	}
	rows := make([]table.Row, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, table.Row{JoinOrNone(labelToName[label]), label})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	if len(rows) == 0 {
		rows = append(rows, table.Row{"none found", "none found"})
	}
	t := NewTable(
		[]table.Column{{Title: strings.Title(m.SelectedType) + " Name(s)", Width: 80}, {Title: "Label Selector", Width: 40}},
		rows,
		m.Prompt,
		15,
	)
	m.Table = t
	m.LabelOptions = labels
	m.LabelToName = labelToName
	m.Step = StepLabel
}
