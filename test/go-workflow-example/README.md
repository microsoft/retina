# Workflow - a library organizes steps with dependencies into DAG (Directed-Acyclic-Graph) for Go
[![Go Report Card](https://goreportcard.com/badge/github.com/Azure/go-workflow)](https://goreportcard.com/report/github.com/Azure/go-workflow)
[![Go Test Status](https://github.com/Azure/go-workflow/actions/workflows/go.yml/badge.svg)](https://github.com/Azure/go-workflow/actions/workflows/go.yml)
[![Go Test Coverage](https://raw.githubusercontent.com/Azure/go-workflow/badges/.badges/main/coverage.svg)](/.github/.testcoverage.yml)

## Overview

> Strongly encourage everyone to read examples in the [example](./example) directory to have a quick understanding of how to use this library.

`go-workflow` helps Go developers organize steps with dependencies into a Directed-Acyclic-Graph (DAG).
- It provides a simple and flexible way to define and execute a workflow.
- It is easy to implement steps and compose them into a composite step.
- It uses **goroutine** to execute steps concurrently.
- It supports **retry**, **timeout**, and other configurations for each step.
- It supports **callbacks** to hook before / after each step.

See it in action:

```go
package yours

import (
    "context"

    flow "github.com/Azure/go-workflow"
)

type Step struct{ Value string }

// All required for a step is `Do(context.Context) error`
func (s *Step) Do(ctx context.Context) error {
    fmt.Println(s.Value)
    return nil
}

func main() {
    // declare steps
    var (
        a = new(Step)
        b = &Step{Value: "B"}
        c = flow.Func("declare from anonymous function", func(ctx context.Context) error {
            fmt.Println("C")
            return nil
        })
    )
    // compose steps into a workflow!
    w := new(flow.Workflow)
    w.Add(
        flow.Step(b).DependsOn(a),     // use DependsOn to define dependencies
        flow.Steps(a, b).DependsOn(c), // final execution order: c -> a -> b

        // other configurations, like retry, timeout, condition, etc.
        flow.Step(c).
            Retry(func(ro *flow.RetryOption) {
                ro.Attempts = 3 // retry 3 times
            }).
            Timeout(10*time.Minute), // timeout after 10 minutes

        // use Input to change step at runtime
        flow.Step(a).Input(func(ctx context.Context, a *Step) error {
            a.Value = "A"
            return nil
        }),
    )
    // execute the workflow and block until all steps are terminated
    err := w.Do(context.Background())
}
```

## Document from AI
You can also check the document from deepwiki: https://deepwiki.com/Azure/go-workflow

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft
trademarks or logos is subject to and must follow
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
