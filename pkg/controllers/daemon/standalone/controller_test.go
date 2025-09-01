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
	utils "github.com/microsoft/retina/pkg/controllers/daemon/standalone/utils"
	"github.com/microsoft/retina/pkg/log"
	sm "github.com/microsoft/retina/pkg/module/metrics/standalone"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerReconcile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup logger
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	assert.NoError(t, err)

	// Mock source
	mockSource := utils.NewMockSource(ctrl)

	// Cache
	cache := standalone.New()

	// Metrics module
	ctx := context.Background()
	metricsModule := sm.InitModule(ctx, nil)

	// Prepopulate cache with an endpoint to simulate deletion
	oldEp := common.NewRetinaEndpoint("old-pod", "default", &common.IPAddresses{IPv4: net.ParseIP("1.1.1.2")})
	require.NoError(t, cache.UpdateRetinaEndpoint(oldEp))

	// New endpoint returned by the source
	newEndpoint := common.NewRetinaEndpoint("new-pod", "default", &common.IPAddresses{IPv4: net.ParseIP("1.1.1.1")})
	mockSource.EXPECT().GetAllEndpoints().Return([]*common.RetinaEndpoint{newEndpoint}, nil)

	// Setup test controller
	cfg := &kcfg.Config{MetricsInterval: 1, EnableCrictl: false}
	controller := New(cfg, cache, metricsModule)
	controller.source = mockSource // inject mock source

	// Run Reconcile
	err = controller.Reconcile(ctx)
	assert.NoError(t, err)

	// Validate cache updates
	cachedIPs := cache.GetAllIPs()
	assert.Len(t, cachedIPs, 1, "only new endpoint should remain in cache")
	assert.Contains(t, cachedIPs, "1.1.1.1")

	// Stop the controller and validate cleanup
	controller.Stop()
	assert.Equal(t, len(controller.cache.GetAllIPs()), 0)
}
