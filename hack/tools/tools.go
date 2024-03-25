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
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/goreleaser/goreleaser"
	_ "github.com/onsi/ginkgo"
	_ "mvdan.cc/gofumpt"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
