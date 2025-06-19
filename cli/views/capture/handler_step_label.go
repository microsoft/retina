package captureviews

// Exported handler for label step
func HandleStepLabel(m *MainModel) {
	selected := m.Table.SelectedRow()
	row := ToLabelRow(selected)
	m.Prompt = "Confirm your selection:"
	if row.LabelSelector != "" {
		m.SelectedLabel = row.LabelSelector
		m.Confirmed = true
		m.Step = StepDone
	}
}
