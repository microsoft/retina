package flow

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type wrappedStep struct{ Steper }
type multiStep struct{ steps []Steper }

func wrap(s Steper) *wrappedStep                  { return &wrappedStep{s} }
func multi(ss ...Steper) *multiStep               { return &multiStep{steps: ss} }
func (w *wrappedStep) Unwrap() Steper             { return w.Steper }
func (w *wrappedStep) String() string             { return strings.ToUpper(String(w.Steper)) }
func (m *multiStep) Unwrap() []Steper             { return m.steps }
func (m *multiStep) Do(ctx context.Context) error { return nil }

func TestHas(t *testing.T) {
	var (
		a  = NoOp("a")
		b  = NoOp("b")
		A  = wrap(a)
		ab = multi(a, b)
	)
	assert.True(t, Has[*NoOpStep](a))
	assert.True(t, Has[*NoOpStep](b))
	assert.True(t, Has[*NoOpStep](A))
	assert.True(t, Has[*NoOpStep](ab))

	assert.False(t, Has[*wrappedStep](a))
	assert.False(t, Has[*wrappedStep](b))
	assert.True(t, Has[*wrappedStep](A))
	assert.False(t, Has[*wrappedStep](ab))

	assert.False(t, Has[*multiStep](a))
	assert.False(t, Has[*multiStep](b))
	assert.False(t, Has[*multiStep](A))
	assert.True(t, Has[*multiStep](ab))

	t.Run("is nil", func(t *testing.T) {
		assert.False(t, Has[*NoOpStep](nil))
		assert.False(t, Has[*wrappedStep](nil))
		assert.False(t, Has[*multiStep](nil))
		assert.False(t, Has[*NoOpStep](wrap(nil)))
		assert.False(t, Has[*NoOpStep](multi(nil, nil)))
		assert.False(t, Has[*NoOpStep](multi()))
	})
}

func TestAs(t *testing.T) {
	var (
		a  = NoOp("a")
		b  = NoOp("b")
		A  = wrap(a)
		ab = multi(a, b)
	)

	t.Run("no wrap", func(t *testing.T) {
		assert.Nil(t, As[*multiStep](a))
	})
	t.Run("single wrap", func(t *testing.T) {
		steps := As[*NoOpStep](A)
		if assert.Len(t, steps, 1) {
			assert.True(t, a == steps[0])
		}
	})
	t.Run("multi wrap", func(t *testing.T) {
		steps := As[*NoOpStep](ab)
		assert.ElementsMatch(t, []Steper{a, b}, steps)
	})
	t.Run("nil step", func(t *testing.T) {
		assert.Nil(t, As[*NoOpStep](nil))
	})
	t.Run("unwrap nil", func(t *testing.T) {
		steps := As[*NoOpStep](&wrappedStep{nil})
		assert.Nil(t, steps)
	})
	t.Run("multi unwrap nil", func(t *testing.T) {
		assert.Nil(t, As[*NoOpStep](&multiStep{nil}))
		assert.Nil(t, As[*NoOpStep](&multiStep{steps: []Steper{nil}}))
	})
}

func TestHasStep(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.False(t, HasStep(nil, nil))
		assert.False(t, HasStep(nil, &NoOpStep{}))
		assert.False(t, HasStep(&NoOpStep{}, nil))
	})
	t.Run("single wrap", func(t *testing.T) {
		var (
			a = NoOp("a")
			A = wrap(a)
		)
		assert.True(t, HasStep(A, a))
		assert.False(t, HasStep(a, A))
	})
	t.Run("multi wrap", func(t *testing.T) {
		var (
			a  = NoOp("a")
			b  = NoOp("b")
			ab = multi(a, b)
		)
		assert.True(t, HasStep(ab, a))
		assert.True(t, HasStep(ab, b))
		assert.False(t, HasStep(a, b))
		assert.False(t, HasStep(b, a))
		assert.False(t, HasStep(a, ab))
		assert.False(t, HasStep(b, ab))
	})
}

func TestString(t *testing.T) {
	var (
		a  = NoOp("a")
		b  = NoOp("b")
		A  = wrap(a)
		ab = multi(a, b)
	)
	assert.Equal(t, "<nil>", String(nil))
	assert.Equal(t, "a", String(a))
	assert.Equal(t, "A", String(A))
	assert.Contains(t, String(ab), "*flow.multiStep")
	assert.Contains(t, String(ab), " {\n\ta\n\tb\n}")
}
