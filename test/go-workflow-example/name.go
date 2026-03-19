package flow

import "fmt"

// Name can rename a Step.
//
//	workflow.Add(
//		Step(a),
//		Name(a, "StepA"),
//	)
//
// Attention: Name will wrap the original Step
func Name(step Steper, name string) Builder {
	return Step(&NamedStep{Name: name, Steper: step})
}

// Names can rename multiple Steps.
//
//	workflow.Add(
//		Names(map[Steper]string{
//			stepA: "A",
//			stepB: "B",
//		},
//	)
func Names(m map[Steper]string) Builder {
	as := AddSteps{}
	for step, name := range m {
		as[&NamedStep{name, step}] = nil
	}
	return as
}

// NameFunc can rename a Step with a runtime function.
func NameFunc(step Steper, fn func() string) Builder {
	return NameStringer(step, stringer(fn))
}

// NameStringer can rename a Step with a fmt.Stringer,
// which allows String() method to be called at runtime.
func NameStringer(step Steper, name fmt.Stringer) Builder {
	return Step(&StringerNamedStep{Name: name, Steper: step})
}

// NamedStep is a wrapper of Steper, it gives your step a name by overriding String() method.
type NamedStep struct {
	Name string
	Steper
}

func (ns *NamedStep) String() string { return ns.Name }
func (ns *NamedStep) Unwrap() Steper { return ns.Steper }

type stringer func() string

func (s stringer) String() string { return s() }

type StringerNamedStep struct {
	Name fmt.Stringer
	Steper
}

func (sns *StringerNamedStep) String() string { return sns.Name.String() }
func (sns *StringerNamedStep) Unwrap() Steper { return sns.Steper }
