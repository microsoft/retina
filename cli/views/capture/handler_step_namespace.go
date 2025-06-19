package captureviews

import (
	"sort"

	"github.com/charmbracelet/bubbles/table"
)

// Exported handler for namespace step
func HandleStepNamespace(m *MainModel) {
	selected := m.Table.SelectedRow()
	row := ToResourceRow(selected)
	m.Prompt = "Select a pod label selector:"
	m.SelectedNamespaceLabel = row.NamespaceLabel
	m.SelectedLabel = ""
	m.NsSelectorForView = row.NamespaceLabel

	// Gather all namespaces matching the selected label
	nsList := m.LabelToNS[row.NamespaceLabel]
	if len(nsList) == 0 {
		nsList = []string{}
	}

	// Collect all pod label selectors for pods in the selected namespaces
	labelSet := make(map[string]struct{})
	labelToName := make(map[string][]string)
	for _, ns := range nsList {
		labels, l2n, err := GetResourceLabels("pod", ns)
		if err != nil {
			continue
		}
		for _, label := range labels {
			labelSet[label] = struct{}{}
			labelToName[label] = append(labelToName[label], l2n[label]...)
		}
	}
	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		labels = []string{"none found"}
		labelToName = map[string][]string{"none found": {}}
	}
	rows := make([]table.Row, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, table.Row{JoinOrNone(labelToName[label]), label})
	}
	if len(rows) == 0 {
		rows = append(rows, table.Row{"none found", "none found"})
	}
	// Sort rows by the left column (Pod Name(s))
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})
	t := NewTable(
		[]table.Column{{Title: "Pod Name(s)", Width: 80}, {Title: "Pod Label Selector", Width: 40}},
		rows,
		m.Prompt,
		15,
	)
	m.Table = t
	m.LabelOptions = labels
	m.LabelToName = labelToName
	m.Step = StepLabel
}
