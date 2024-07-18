// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

// NOTE: we could reference this file Cilium's code, but it is a small file.
// Referencing Cilium's code requires importing dependencies
// we don't need from their operator, which has BGP dependencies for instance.
// This is currently resulting in an error in go mod tidy:
// module go.universe.tf/metallb@latest found (v0.13.12), but does not contain package go.universe.tf/metallb/pkg/speaker

package ciliumcrds

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ProviderFlagsHooks interface {
	RegisterProviderFlag(cmd *cobra.Command, vp *viper.Viper)
}
