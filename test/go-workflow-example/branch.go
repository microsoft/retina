package flow

import (
	"context"
)

// BranchCheckFunc checks the target and returns true if the branch should be selected.
type BranchCheckFunc[T Steper] func(context.Context, T) (bool, error)

// If adds a conditional branch to the workflow.
//
//	If(someStep, func(ctx context.Context, someStep *SomeStep) (bool, error) {
//		// branch condition here, true -> Then, false -> Else.
//		// if error is returned, then fail the selected branch step.
//	}).
//	Then(thenStep).
//	Else(elseStep)
func If[T Steper](target T, check BranchCheckFunc[T]) *IfBranch[T] {
	return &IfBranch[T]{Target: target, BranchCheck: BranchCheck[T]{Check: check}}
}

// IfBranch adds target step, then and else step to workflow,
// and check the target step and determine which branch to go.
type IfBranch[T Steper] struct {
	Target      T // the target to check
	BranchCheck BranchCheck[T]
	ThenStep    []Steper
	ElseStep    []Steper
	Cond        Condition // Cond is the When condition for both ThenStep and ElseStep, not target Step!
}

// Then adds steps to the Then branch.
func (i *IfBranch[T]) Then(th ...Steper) *IfBranch[T] {
	i.ThenStep = append(i.ThenStep, th...)
	return i
}

// Else adds steps to the Else branch.
func (i *IfBranch[T]) Else(el ...Steper) *IfBranch[T] {
	i.ElseStep = append(i.ElseStep, el...)
	return i
}

// When adds a condition to both Then and Else steps, not the Target!
// Default to DefaultCondition.
func (i *IfBranch[T]) When(cond Condition) *IfBranch[T] {
	i.Cond = cond
	return i
}
func (i *IfBranch[T]) isThen(isThen bool) Condition {
	return func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
		if status := ConditionOrDefault(i.Cond)(ctx, ups); status != Running {
			return status
		}
		if i.BranchCheck.OK == isThen {
			return Running
		}
		return Skipped
	}
}
func (i *IfBranch[T]) AddToWorkflow() map[Steper]*StepConfig {
	return Steps().Merge(
		Steps(i.Target).AfterStep(func(ctx context.Context, s Steper, err error) error {
			i.BranchCheck.Do(ctx, i.Target)
			return err
		}),
		Steps(i.ThenStep...).When(i.isThen(true)),
		Steps(i.ElseStep...).When(i.isThen(false)),
		Steps(append(append([]Steper{},
			i.ThenStep...), i.ElseStep...,
		)...).
			DependsOn(i.Target).
			BeforeStep(func(ctx context.Context, s Steper) (context.Context, error) {
				if i.BranchCheck.Error != nil {
					return ctx, i.BranchCheck.Error
				}
				return ctx, nil
			}),
	).AddToWorkflow()
}

// Switch adds a switch branch to the workflow.
//
//	Switch(someStep).
//		Case(case1, func(ctx context.Context, someStep *SomeStep) (bool, error) {
//			// branch condition here, true to select this branch
//			// error will fail the case
//		}).
//		Default(defaultStep), // the step to run if all case checks return false
//	)
func Switch[T Steper](target T) *SwitchBranch[T] {
	return &SwitchBranch[T]{Target: target, CasesToCheck: make(map[Steper]*BranchCheck[T])}
}

// SwitchBranch adds target step, cases and default step to workflow,
// and check the target step and determine which branch to go.
type SwitchBranch[T Steper] struct {
	Target       T
	CasesToCheck map[Steper]*BranchCheck[T]
	DefaultStep  []Steper
	Cond         Condition
}

// BranchCheck represents a branch to be checked.
type BranchCheck[T Steper] struct {
	Check BranchCheckFunc[T]
	OK    bool
	Error error
}

func (bc *BranchCheck[T]) Do(ctx context.Context, target T) {
	bc.OK, bc.Error = bc.Check(ctx, target)
}

// Case adds a case to the switch branch.
func (s *SwitchBranch[T]) Case(step Steper, check BranchCheckFunc[T]) *SwitchBranch[T] {
	return s.Cases([]Steper{step}, check)
}

// Cases adds multiple cases to the switch branch.
// The check function will be executed for each case step.
func (s *SwitchBranch[T]) Cases(steps []Steper, check BranchCheckFunc[T]) *SwitchBranch[T] {
	for _, step := range steps {
		s.CasesToCheck[step] = &BranchCheck[T]{Check: check}
	}
	return s
}

// Default adds default step(s) to the switch branch.
func (s *SwitchBranch[T]) Default(step ...Steper) *SwitchBranch[T] {
	s.DefaultStep = append(s.DefaultStep, step...)
	return s
}

// When adds a condition to all case steps and default, not the Target!
func (s *SwitchBranch[T]) When(cond Condition) *SwitchBranch[T] {
	s.Cond = cond
	return s
}
func (s *SwitchBranch[T]) isCase(c Steper) func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	return func(ctx context.Context, ups map[Steper]StepResult) StepStatus {
		if status := ConditionOrDefault(s.Cond)(ctx, ups); status != Running {
			return status
		}
		if check, ok := s.CasesToCheck[c]; ok {
			check.Do(ctx, s.Target)
			if check.OK {
				return Running
			}
		}
		return Skipped
	}
}
func (s *SwitchBranch[T]) isDefault(ctx context.Context, ups map[Steper]StepResult) StepStatus {
	for _, check := range s.CasesToCheck {
		if check.OK {
			return Skipped
		}
	}
	// default branch ignores the status from cases
	up := make(map[Steper]StepResult)
	for step, status := range ups {
		if _, isCase := s.CasesToCheck[step]; !isCase {
			up[step] = status
		}
	}
	if status := ConditionOrDefault(s.Cond)(ctx, up); status != Running {
		return status
	}
	return Running
}
func (s *SwitchBranch[T]) AddToWorkflow() map[Steper]*StepConfig {
	steps := Steps()
	cases := []Steper{}
	for step := range s.CasesToCheck {
		step := step
		cases = append(cases, step)
		steps.Merge(
			Steps(step).
				DependsOn(s.Target).
				When(s.isCase(step)).
				BeforeStep(func(ctx context.Context, step Steper) (context.Context, error) {
					for c, check := range s.CasesToCheck {
						if HasStep(step, c) && check.Error != nil {
							return ctx, check.Error
						}
					}
					return ctx, nil
				}),
		)
	}
	if s.DefaultStep != nil {
		steps.Merge(
			Steps(s.DefaultStep...).
				DependsOn(s.Target).
				DependsOn(cases...).
				When(s.isDefault),
		)
	}
	return steps.AddToWorkflow()
}
