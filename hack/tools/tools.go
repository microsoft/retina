//go:build tools
// +build tools

/*
How to add new tool:

$ cd hack/tools
$ export GOBIN=$PWD/bin
$ export PATH=$GOBIN:$PATH
$ go install ...
*/

package tools

import (
	_ "github.com/goreleaser/goreleaser"
	_ "github.com/onsi/ginkgo"
	_ "go.uber.org/mock/mockgen"
	_ "mvdan.cc/gofumpt"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
