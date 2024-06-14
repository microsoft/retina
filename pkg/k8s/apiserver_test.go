package k8s

import (
	"testing"

	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNonCacheEvent(_ *testing.T) {
	a := newAPIServerEventHandler(params{
		Logger: logrus.New(),
	})
	a.handleAPIServerEvent("test")
}

func TestHandler(t *testing.T) {
	a := newAPIServerEventHandler(params{
		Logger:  logrus.New(),
		IPCache: ipcache.NewIPCache(&ipcache.Configuration{}),
	})

	// Add API server IPs.
	a.handleAPIServerEvent(&cc.CacheEvent{
		Type: cc.EventTypeAddAPIServerIPs,
		Obj:  common.NewAPIServerObject([]string{"52.0.0.1"}),
	})

	ip, ok := a.c.LookupByIP("52.0.0.1")
	assert.True(t, ok)
	assert.Equal(t, identity.ReservedIdentityKubeAPIServer, ip.ID)

	// Delete API server IPs.
	a.handleAPIServerEvent(&cc.CacheEvent{
		Type: cc.EventTypeDeleteAPIServerIPs,
		Obj:  common.NewAPIServerObject([]string{"52.0.0.1"}),
	})
	_, ok = a.c.LookupByIP("52.0.0.1")
	assert.False(t, ok)
}
