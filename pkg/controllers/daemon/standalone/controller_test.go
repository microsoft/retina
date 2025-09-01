// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"context"
	"net"
	"testing"

	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache/standalone"
	"github.com/microsoft/retina/pkg/controllers/daemon/standalone/source"
	"github.com/microsoft/retina/pkg/log"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerReconcile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup logger
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	// Mock source
	mockSource := source.NewMockSource(ctrl)

	// Cache
	cache := standalone.New()

	// Metrics module
	ctx := context.Background()
	// nolint:gocritic
	// metricsModule := sm.InitModule(ctx, nil)

	// Prepopulate cache with an endpoint to simulate deletion
	oldEp := common.NewRetinaEndpoint("old-pod", "default", &common.IPAddresses{IPv4: net.ParseIP("1.1.1.2")})
	require.NoError(t, cache.UpdateRetinaEndpoint(oldEp))

	// New endpoint returned by the source
	newEndpoint := common.NewRetinaEndpoint("new-pod", "default", &common.IPAddresses{IPv4: net.ParseIP("1.1.1.1")})
	mockSource.EXPECT().GetAllEndpoints().Return([]*common.RetinaEndpoint{newEndpoint}, nil)

	// Setup test controller
	cfg := &kcfg.Config{MetricsInterval: 1, EnableCrictl: false}
	// nolint:gocritic
	// controller := New(cfg, cache, metricsModule)
	controller := New(cfg, cache)
	controller.src = mockSource // inject mock source

	// Run Reconcile
	err = controller.Reconcile(ctx)
	require.NoError(t, err)

	// Validate cache updates
	cachedIPs := cache.GetAllIPs()
	require.Len(t, cachedIPs, 1, "only new endpoint should remain in cache")
	require.Contains(t, cachedIPs, "1.1.1.1")

	// Stop the controller and validate cleanup
	controller.Stop()
	require.Empty(t, controller.cache.GetAllIPs())
}
