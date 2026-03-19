package flow_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIf(t *testing.T) {
	type mockIf struct {
		*mock.Mock
		Target, Then, Else flow.Steper
	}

	newMockIf := func() *mockIf {
		var (
			m    = new(mock.Mock)
			step = flow.Func("Target", func(ctx context.Context) error {
				m.MethodCalled("Target")
				return nil
			})
			thenStep = flow.Func("Then", func(ctx context.Context) error {
				m.MethodCalled("Then")
				return nil
			})
			elseStep = flow.Func("Else", func(ctx context.Context) error {
				m.MethodCalled("Else")
				return nil
			})
		)
		return &mockIf{m, step, thenStep, elseStep}
	}
	t.Run("Check should happen after target done, and before then / else", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, s flow.Steper) (bool, error) {
				m.MethodCalled("Check")
				return false, nil
			}).Then(m.Then).Else(m.Else),
		)
		var (
			mTarget = m.On("Target")
			mCheck  = m.On("Check").NotBefore(mTarget)
			_       = m.On("Else").NotBefore(mTarget, mCheck)
		)
		assert.NoError(t, w.Do(context.Background()))
	})
	t.Run("Check returns true, nil, then goto Then", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, f flow.Steper) (bool, error) {
				return true, nil
			}).Then(m.Then).Else(m.Else),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Then").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertCalled(t, "Then")
			m.AssertNotCalled(t, "Else")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Then).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Else).Status)
		}
	})
	t.Run("Check returns false, nil, then goto Else", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, s flow.Steper) (bool, error) {
				return false, nil
			}).Then(m.Then).Else(m.Else),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Else").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertNotCalled(t, "Then")
			m.AssertCalled(t, "Else")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Then).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Else).Status)
		}
	})
	t.Run("return error then fail the branch", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, s flow.Steper) (bool, error) {
				return false, assert.AnError
			}).Then(m.Then).Else(m.Else),
		)
		m.On("Target")
		if assert.Error(t, w.Do(context.Background())) {
			m.AssertNotCalled(t, "Then")
			m.AssertNotCalled(t, "Else")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Then).Status)
			assert.Equal(t, flow.Failed, w.StateOf(m.Else).Status)
		}
	})
	t.Run("when can change then and else condition", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, s flow.Steper) (bool, error) {
				return true, nil
			}).Then(m.Then).Else(m.Else).When(flow.AnyFailed),
		)
		m.On("Target")
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertNotCalled(t, "Then")
			m.AssertNotCalled(t, "Else")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Then).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Else).Status)
		}
	})
	t.Run("still works even the original step is replaced", func(t *testing.T) {
		m := newMockIf()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.If(m.Target, func(ctx context.Context, s flow.Steper) (bool, error) {
				return true, nil
			}).Then(m.Then).Else(m.Else),
			flow.Names(map[flow.Steper]string{
				m.Target: "Target",
				m.Then:   "Then",
				m.Else:   "Else",
			}),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Then").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertCalled(t, "Then")
			m.AssertNotCalled(t, "Else")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Then).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Else).Status)
		}
	})
}

func TestSwitch(t *testing.T) {
	type mockSwitch struct {
		*mock.Mock
		Target, Case1, Case2, Default flow.Steper
	}
	newMockSwitch := func() *mockSwitch {
		var (
			m    = new(mock.Mock)
			step = flow.Func("Target", func(ctx context.Context) error {
				m.MethodCalled("Target")
				return nil
			})
			case1 = flow.Func("Case1", func(ctx context.Context) error {
				m.MethodCalled("Case1")
				return nil
			})
			case2 = flow.Func("Case2", func(ctx context.Context) error {
				m.MethodCalled("Case2")
				return nil
			})
			def = flow.Func("Default", func(ctx context.Context) error {
				m.MethodCalled("Default")
				return nil
			})
		)
		return &mockSwitch{m, step, case1, case2, def}
	}
	t.Run("run selected case", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, nil
				}).
				Case(m.Case2, func(ctx context.Context, s flow.Steper) (bool, error) {
					return false, nil
				}).
				Default(m.Default),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Case1").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertCalled(t, "Case1")
			m.AssertNotCalled(t, "Case2")
			m.AssertNotCalled(t, "Default")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Case1).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Case2).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Default).Status)
		}
	})
	t.Run("multiple cases can be selected at the same time", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, nil
				}).
				Case(m.Case2, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, nil
				}).
				Default(m.Default),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Case1").NotBefore(mTarget)
			_       = m.On("Case2").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertCalled(t, "Case1")
			m.AssertCalled(t, "Case2")
			m.AssertNotCalled(t, "Default")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Case1).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Case2).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Default).Status)
		}
	})
	t.Run("if no case selected, fallback to default", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
					return false, nil
				}).
				Case(m.Case2, func(ctx context.Context, s flow.Steper) (bool, error) {
					return false, nil
				}).
				Default(m.Default),
		)
		var (
			mTarget = m.On("Target")
			_       = m.On("Default").NotBefore(mTarget)
		)
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertNotCalled(t, "Case1")
			m.AssertNotCalled(t, "Case2")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Case1).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Case2).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Default).Status)
		}
	})
	t.Run("case being failed if check returns error", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, assert.AnError
				}),
		)
		m.On("Target")
		var err flow.ErrWorkflow
		if assert.ErrorAs(t, w.Do(context.Background()), &err) {
			assert.Equal(t, flow.Failed, err[m.Case1].Status)
			assert.ErrorIs(t, err[m.Case1].Err, assert.AnError)
		}
	})
	t.Run("cases will select multiple cases at the same time", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Cases([]flow.Steper{m.Case1, m.Case2}, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, nil
				}),
		)
		target := m.On("Target")
		m.On("Case1").NotBefore(target)
		m.On("Case2").NotBefore(target)
		if assert.NoError(t, w.Do(context.Background())) {
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Case1).Status)
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Case2).Status)
		}
	})
	t.Run("when will set condition for all cases and default", func(t *testing.T) {
		m := newMockSwitch()
		defer m.AssertExpectations(t)
		w := new(flow.Workflow).Add(
			flow.Switch(m.Target).
				Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
					return true, nil
				}).
				Case(m.Case2, func(ctx context.Context, s flow.Steper) (bool, error) {
					return false, nil
				}).
				Default(m.Default).
				When(flow.AnyFailed),
		)
		m.On("Target")
		if assert.NoError(t, w.Do(context.Background())) {
			m.AssertNotCalled(t, "Case1")
			m.AssertNotCalled(t, "Case2")
			m.AssertNotCalled(t, "Default")
			assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Case1).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Case2).Status)
			assert.Equal(t, flow.Skipped, w.StateOf(m.Default).Status)
		}
	})
	t.Run("replace original step", func(t *testing.T) {
		t.Run("still works", func(t *testing.T) {
			m := newMockSwitch()
			defer m.AssertExpectations(t)
			w := new(flow.Workflow).Add(
				flow.Switch(m.Target).
					Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
						return true, nil
					}).
					Case(m.Case2, func(ctx context.Context, s flow.Steper) (bool, error) {
						return false, nil
					}).
					Default(m.Default),
				flow.Names(map[flow.Steper]string{
					m.Target:  "Target",
					m.Case1:   "Case1",
					m.Case2:   "Case2",
					m.Default: "Default",
				}),
			)
			var (
				mTarget = m.On("Target")
				_       = m.On("Case1").NotBefore(mTarget)
			)
			if assert.NoError(t, w.Do(context.Background())) {
				m.AssertCalled(t, "Case1")
				m.AssertNotCalled(t, "Case2")
				m.AssertNotCalled(t, "Default")
				assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
				assert.Equal(t, flow.Succeeded, w.StateOf(m.Case1).Status)
				assert.Equal(t, flow.Skipped, w.StateOf(m.Case2).Status)
				assert.Equal(t, flow.Skipped, w.StateOf(m.Default).Status)
			}
		})
		t.Run("still fail the case if check returns error", func(t *testing.T) {
			m := newMockSwitch()
			defer m.AssertExpectations(t)
			w := new(flow.Workflow).Add(
				flow.Switch(m.Target).
					Case(m.Case1, func(ctx context.Context, s flow.Steper) (bool, error) {
						return true, assert.AnError
					}).
					Default(m.Default),
				flow.Name(m.Case1, "Case1"),
			)
			m.On("Target")
			if assert.Error(t, w.Do(context.Background())) {
				m.AssertNotCalled(t, "Case1")
				m.AssertNotCalled(t, "Default")
				assert.Equal(t, flow.Succeeded, w.StateOf(m.Target).Status)
				assert.Equal(t, flow.Failed, w.StateOf(m.Case1).Status)
				assert.Equal(t, flow.Skipped, w.StateOf(m.Default).Status)
			}
		})
	})
}
