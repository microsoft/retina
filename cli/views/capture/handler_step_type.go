package captureviews

import (
	"sort"

	"github.com/charmbracelet/bubbles/table"
)

// Exported handler for resource type step
func HandleStepKubernetesResourceType(m *MainModel) {
	selected := m.Table.SelectedRow()
	row := ToResourceRow(selected)
	if row.NamespaceNames != "" {
		m.SelectedType = row.NamespaceNames // Save resource type (Pod, Deployment, etc.)
		m.Prompt = "Select a namespace label selector:"
		labels, labelToNS, err := GetNamespaceLabels()
		if err != nil || len(labels) == 0 {
			labels = []string{"none found"}
		}
		rows := make([]table.Row, 0, len(labels))
		for _, label := range labels {
			if nsList, ok := labelToNS[label]; ok && len(nsList) > 0 {
				rows = append(rows, table.Row{JoinOrNone(nsList), label})
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
		if len(rows) == 0 {
			rows = append(rows, table.Row{"none found", "none found"})
		}
		cols := []table.Column{{Title: "Namespace(s)", Width: 80}, {Title: "Namespace Label Selector", Width: 40}}
		t := NewTable(cols, rows, m.Prompt, 15)
		m.Table = t
		m.LabelOptions = labels
		m.LabelToNS = labelToNS
		m.Step = StepNamespace
	}
}
