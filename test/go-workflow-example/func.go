package flow

import (
	"context"
)

// Func constructs a Step from an arbitrary function
func Func(name string, do func(context.Context) error) *Function[struct{}, struct{}] {
	return FuncIO(name, func(ctx context.Context, _ struct{}) (struct{}, error) {
		return struct{}{}, do(ctx)
	})
}
func FuncIO[I, O any](name string, do func(context.Context, I) (O, error)) *Function[I, O] {
	f := &Function[I, O]{Name: name, DoFunc: do}
	return f
}
func FuncI[I any](name string, do func(context.Context, I) error) *Function[I, struct{}] {
	return FuncIO(name, func(ctx context.Context, i I) (struct{}, error) {
		return struct{}{}, do(ctx, i)
	})
}
func FuncO[O any](name string, do func(context.Context) (O, error)) *Function[struct{}, O] {
	return FuncIO(name, func(ctx context.Context, _ struct{}) (O, error) {
		return do(ctx)
	})
}

// Function wraps an arbitrary function as a Step.
type Function[I, O any] struct {
	Name   string
	Input  I
	Output O
	DoFunc func(context.Context, I) (O, error)
}

func (f *Function[I, O]) String() string { return f.Name }
func (f *Function[I, O]) Do(ctx context.Context) error {
	var err error
	if f.DoFunc != nil {
		f.Output, err = f.DoFunc(ctx, f.Input)
	}
	return err
}
