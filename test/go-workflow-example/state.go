package flow

import (
	"context"
	"sync"
)

// State is the internal state of a Step in a Workflow.
//
// It has the status and the config (dependency, input, retry option, condition, timeout, etc.) of the step.
// The status could be read / write from different goroutines, so use RWMutex to protect it.
type State struct {
	StepResult
	Config *StepConfig
	sync.RWMutex
}

func (s *State) GetStatus() StepStatus {
	s.RLock()
	defer s.RUnlock()
	return s.Status
}
func (s *State) SetStatus(ss StepStatus) {
	s.Lock()
	defer s.Unlock()
	s.Status = ss
}
func (s *State) GetError() error {
	s.RLock()
	defer s.RUnlock()
	return s.Err
}
func (s *State) SetError(err error) {
	s.Lock()
	defer s.Unlock()
	s.Err = err
}
func (s *State) GetStepResult() StepResult {
	s.RLock()
	defer s.RUnlock()
	return s.StepResult
}
func (s *State) Upstreams() Set[Steper] {
	if s.Config == nil {
		return nil
	}
	return s.Config.Upstreams
}
func (s *State) Option() *StepOption {
	opt := &StepOption{}
	if s.Config != nil && s.Config.Option != nil {
		for _, o := range s.Config.Option {
			o(opt)
		}
	}
	return opt
}
func (s *State) Before(root context.Context, step Steper, do func(func() error) error) (context.Context, error) {
	if s.Config == nil || len(s.Config.Before) == 0 {
		return root, nil
	}
	ctx := root
	for _, b := range s.Config.Before {
		if err := do(func() error {
			ctxReturned, err := b(ctx, step)
			ctx = ctxReturned // use the context returned by Before callback
			return err
		}); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
func (s *State) After(ctx context.Context, step Steper, err error) error {
	if s.Config == nil || len(s.Config.After) == 0 {
		return err
	}
	for _, a := range s.Config.After {
		err = a(ctx, step, err)
	}
	return err
}
func (s *State) AddUpstream(up Steper) {
	if s.Config == nil {
		s.Config = &StepConfig{}
	}
	if s.Config.Upstreams == nil {
		s.Config.Upstreams = make(Set[Steper])
	}
	if up != nil {
		s.Config.Upstreams.Add(up)
	}
}
func (s *State) MergeConfig(sc *StepConfig) {
	if s.Config == nil {
		s.Config = &StepConfig{}
	}
	s.Config.Merge(sc)
}
