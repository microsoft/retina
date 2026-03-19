package flow_test

import (
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
)

// # Branch: If / Switch
//
// Based on the condition, now it's possible to add branch control to the workflow.
//
// Introduce `If` and `Switch`, they're not steps,
// rather a control branch that add into workflow and manages the condition of their branch steps.
func ExampleIf() {
	var (
		item       string
		isNotEmpty = flow.FuncO("IsNotEmpty", func(ctx context.Context) (bool, error) {
			return item != "", nil
		})
		newIt = flow.Func("NewIt", func(ctx context.Context) error {
			item = "new"
			return nil
		})
		updateIt = flow.Func("UpdateIt", func(ctx context.Context) error {
			item += "_updated"
			return nil
		})
	)
	w := new(flow.Workflow).Add(
		flow.If(isNotEmpty, func(ctx context.Context, f *flow.Function[struct{}, bool]) (bool, error) {
			return f.Output, nil
		}).
			Then(updateIt).
			Else(newIt),
	)
	fmt.Println(item) //
	w.Do(context.Background())
	fmt.Println(item) // new
	w.Do(context.Background())
	fmt.Println(item) // new_updated
	// Output:
	//
	// new
	// new_updated
}

func ExampleSwitch() {
	var (
		age    int
		getAge = flow.Func("GetAge", func(ctx context.Context) error {
			age = 20
			return nil
		})
		canDrive  = Print("CanDrive")
		canDrink  = Print("CanDrink")
		canOwnGun = Print("CanOwnGun")
	)
	w := new(flow.Workflow).Add(
		flow.Switch(getAge).
			Case(canDrive, func(ctx context.Context, f *flow.Function[struct{}, struct{}]) (bool, error) {
				return age >= 16, nil
			}).
			Case(canDrink, func(ctx context.Context, f *flow.Function[struct{}, struct{}]) (bool, error) {
				return age >= 21, nil
			}).
			Case(canOwnGun, func(ctx context.Context, f *flow.Function[struct{}, struct{}]) (bool, error) {
				return age >= 18, nil
			}),

		flow.Step(canOwnGun).DependsOn(canDrive), // just let them print in order
	)
	w.Do(context.Background())
	fmt.Println(w.StateOf(canDrink).Status) // Skipped
	// Output:
	// CanDrive
	// CanOwnGun
	// Skipped
}
