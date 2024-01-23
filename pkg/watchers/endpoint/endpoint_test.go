// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package endpoint

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestGetWatcher(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	v := Watcher()
	assert.NotNil(t, v)

	v_again := Watcher()
	assert.Equal(t, v, v_again, "Expected the same veth watcher instance")
}

func TestEndpointWatcherStart(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := context.Background()

	// When veth is already running.
	v := &EndpointWatcher{
		isRunning: true,
		l:         log.Logger().Named("veth-watcher"),
	}
	err := v.Init(c)
	assert.NoError(t, err, "Expected no error when starting a running veth watcher")
	assert.Equal(t, true, v.isRunning, "Expected veth watcher to be running")

	// When veth is not running.
	v.isRunning = false
	err = v.Init(c)
	assert.NoError(t, err, "Expected no error when starting a stopped veth watcher")
	assert.Equal(t, true, v.isRunning, "Expected veth watcher to be running")

	// Stop the watcher.
	err = v.Stop(c)
	assert.NoError(t, err, "Expected no error when stopping a running veth watcher")

	// Restart the watcher.
	err = v.Init(c)
	assert.NoError(t, err, "Expected no error when starting a stopped veth watcher")
	assert.Equal(t, true, v.isRunning, "Expected veth watcher to be running")

	// Stop the watcher.
	err = v.Stop(c)
	assert.NoError(t, err, "Expected no error when stopping a running veth watcher")
}

func TestEndpointWatcherStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := context.Background()

	// When veth is already stopped.
	v := &EndpointWatcher{
		isRunning: false,
		l:         log.Logger().Named("veth-watcher"),
	}
	err := v.Stop(c)
	assert.NoError(t, err, "Expected no error when stopping a stopped veth watcher")
	assert.Equal(t, false, v.isRunning, "Expected veth watcher to be stopped")

	// Start the watcher.
	err = v.Init(c)
	assert.NoError(t, err, "Expected no error when starting a stopped veth watcher")

	// Stop the watcher.
	err = v.Stop(c)
	assert.NoError(t, err, "Expected no error when stopping a running veth watcher")
	assert.Equal(t, false, v.isRunning, "Expected veth watcher to be stopped")
}

func TestRun(t *testing.T) {
	showLink = func() ([]netlink.Link, error) {
		return []netlink.Link{
			&netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth0",
				},
			},
			&netlink.Vxlan{
				LinkAttrs: netlink.LinkAttrs{
					Name: "eth0",
				},
			},
		}, nil
	}

	links, err := listVeths()
	assert.NoError(t, err, "Expected no error when listing veths")
	assert.Equal(t, 1, len(links), "Expected to find 1 veth")
	assert.Equal(t, "veth0", links[0].Attrs().Name, "Expected to find veth0")
}

func TestDiffCache(t *testing.T) {
	old := cache{
		key{
			name:         "veth0",
			hardwareAddr: "00:00:00:00:00:00",
			netNsID:      0,
		}: netlink.LinkAttrs{
			Name: "veth0",
		},
	}
	new := cache{
		key{
			name:         "veth1",
			hardwareAddr: "00:00:00:00:00:FF",
			netNsID:      1,
		}: netlink.LinkAttrs{
			Name: "veth1",
		},
	}
	e := &EndpointWatcher{current: old, new: new}
	c, d := e.diffCache()
	assert.Equal(t, 1, len(c), "Expected to find 1 created veth")
	assert.Equal(t, 1, len(d), "Expected to find 1 deleted veth")
	assert.Equal(t, "veth1", c[0].(netlink.LinkAttrs).Name, "Expected to find veth1")
	assert.Equal(t, "veth0", d[0].(netlink.LinkAttrs).Name, "Expected to find veth0")
}

func TestRefreshAndCallback(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := context.Background()

	showLink = func() ([]netlink.Link, error) {
		return []netlink.Link{
			&netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth0",
					HardwareAddr: func() net.HardwareAddr {
						mac, _ := net.ParseMAC("00:00:00:00:00:00")
						return mac
					}(),
					NetNsID: 0,
				},
			},
			&netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth1",
					HardwareAddr: func() net.HardwareAddr {
						mac, _ := net.ParseMAC("00:00:00:00:00:01")
						return mac
					}(),
					NetNsID: 1,
				},
			},
		}, nil
	}

	cache := make(cache)
	cache[key{
		name:         "veth2",
		hardwareAddr: "00:00:00:00:00:02",
		netNsID:      2,
	}] = &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "veth2",
		},
	}

	v := &EndpointWatcher{
		isRunning: true,
		current:   cache,
		l:         log.Logger().Named("veth-watcher"),
		p:         pubsub.New(),
	}

	// When cache is empty.
	assert.Equal(t, 1, len(v.current), "Expected to find 0 veths")

	// Post refresh.
	err := v.Refresh(c)
	assert.NoError(t, err, "Expected no error when refreshing veth cache")
	assert.Equal(t, 2, len(v.current), "Expected to find 2 veths")
	assert.Equal(t, "veth0", v.current[key{
		name:         "veth0",
		hardwareAddr: "00:00:00:00:00:00",
		netNsID:      0,
	}].(netlink.LinkAttrs).Name, "Expected to find veth0")
	assert.Equal(t, "veth1", v.current[key{
		name:         "veth1",
		hardwareAddr: "00:00:00:00:00:01",
		netNsID:      1,
	}].(netlink.LinkAttrs).Name, "Expected to find veth1")
}

func TestRefreshError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := context.Background()

	showLink = func() ([]netlink.Link, error) {
		return nil, errors.New("error")
	}

	v := &EndpointWatcher{
		isRunning: true,
		current:   make(cache),
		l:         log.Logger().Named("veth-watcher"),
		p:         pubsub.New(),
	}

	err := v.Refresh(c)
	assert.Error(t, err, "Expected an error when refreshing veth cache")
}

func TestListVethsError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	showLink = func() ([]netlink.Link, error) {
		return nil, errors.New("error")
	}

	_, err := listVeths()
	assert.Error(t, err, "Expected an error when listing veths")
}
