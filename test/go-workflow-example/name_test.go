package flow_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	t.Run("Name can rename a step", func(t *testing.T) {
		var (
			w = new(flow.Workflow)
			s = flow.NoOp("oldName")
		)
		w.Add(
			flow.Step(s),
			flow.Name(s, "newName"),
		)
		steps := w.Steps()
		if assert.Len(t, steps, 1) {
			assert.Equal(t, "newName", flow.String(steps[0]))
		}
	})
	t.Run("Names can rename multiple steps", func(t *testing.T) {
		var (
			w = new(flow.Workflow)
			a = flow.NoOp("a")
			b = flow.NoOp("b")
			c = flow.NoOp("c")
		)
		w.Add(
			flow.Steps(a, b, c),
			flow.Names(map[flow.Steper]string{
				a: "A", b: "B", c: "C",
			}),
		)
		steps := w.Steps()
		if assert.Len(t, steps, 3) {
			names := []string{}
			for _, s := range steps {
				names = append(names, flow.String(s))
			}
			assert.ElementsMatch(t, []string{"A", "B", "C"}, names)
		}
	})
	t.Run("NameFunc can rename a step with runtime String()", func(t *testing.T) {
		var (
			w    = new(flow.Workflow)
			name = ""
			a    = flow.Func("a", func(context.Context) error {
				name = "A"
				return nil
			})
		)
		w.Add(
			flow.Step(a),
			flow.NameFunc(a, func() string {
				return name
			}),
		)
		assert.NoError(t, w.Do(context.Background()))
		steps := w.Steps()
		if assert.Len(t, steps, 1) {
			assert.Equal(t, "A", flow.String(steps[0]))
		}
	})
}
