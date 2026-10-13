// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

//go:build linux

package ciliumcrds

import (
	"io"
	"log/slog"
	"testing"

	"github.com/cilium/cilium/operator/auth/identity"
	ztunnelConfig "github.com/cilium/cilium/operator/pkg/ztunnel/config"
	"github.com/cilium/cilium/pkg/hive"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	k8sTestutils "github.com/cilium/cilium/pkg/k8s/client/testutils"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/hive/cell"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationGraph(t *testing.T) {
	var (
		cfg      ztunnelConfig.Config
		provider identity.Provider
	)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := hive.New(
		authenticationCell,
		cell.Provide(
			// v1.20's ztunnel.Cell builds the namespace reflector at Populate
			// time, which calls Clientset.IsEnabled — nil panics.
			func() k8sClient.Clientset { _, cs := k8sTestutils.NewFakeClientset(logger); return cs },
			func() resource.Resource[*ciliumv2.CiliumIdentity] { return nil },
		),
		cell.Invoke(func(ztunnelCfg ztunnelConfig.Config, identityProvider identity.Provider) {
			cfg = ztunnelCfg
			provider = identityProvider
		}),
	)

	require.NoError(t, h.Populate(logger))
	require.False(t, cfg.EnableZTunnel)
	require.NotNil(t, provider)
}
