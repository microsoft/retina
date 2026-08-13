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
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/hive"
	"github.com/cilium/hive/cell"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationGraph(t *testing.T) {
	var (
		cfg      ztunnelConfig.Config
		provider identity.Provider
	)

	h := hive.New(
		authenticationCell,
		cell.Provide(
			func() k8sClient.Clientset { return nil },
			func() resource.Resource[*ciliumv2.CiliumIdentity] { return nil },
		),
		cell.Invoke(func(ztunnelCfg ztunnelConfig.Config, identityProvider identity.Provider) {
			cfg = ztunnelCfg
			provider = identityProvider
		}),
	)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, h.Populate(logger))
	require.False(t, cfg.EnableZTunnel)
	require.NotNil(t, provider)
}
