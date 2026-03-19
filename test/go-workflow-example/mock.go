package flow

import "context"

// Mock helps to mock a step in Workflow.
//
//	w.Add(
//		flow.Mock(step, func(ctx context.Context) error {}),
//	)
func Mock[T Steper](step T, do func(context.Context) error) Builder {
	return Step(&MockStep{Step: step, MockDo: do})
}

// MockStep helps to mock a step.
// After building a workflow, you can mock the original step with a mock step.
type MockStep struct {
	Step   Steper
	MockDo func(context.Context) error
}

func (m *MockStep) Unwrap() Steper               { return m.Step }
func (m *MockStep) Do(ctx context.Context) error { return m.MockDo(ctx) }
