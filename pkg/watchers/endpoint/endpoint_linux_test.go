// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package endpoint

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	"golang.org/x/sync/errgroup"
)

func TestGetWatcher(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	w1 := NewWatcher()
	assert.NotNil(t, w1)

	w2 := NewWatcher()
	assert.NotNil(t, w2)
	assert.Equal(t, w1, w2, "Expected the same veth watcher instance")
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
	e := &Watcher{current: old, new: new, refreshRate: 5 * time.Second}
	c, d := e.diffCache()
	assert.Equal(t, 1, len(c), "Expected to find 1 created veth")
	assert.Equal(t, 1, len(d), "Expected to find 1 deleted veth")
	assert.Equal(t, "veth1", c[0].(netlink.LinkAttrs).Name, "Expected to find veth1")
	assert.Equal(t, "veth0", d[0].(netlink.LinkAttrs).Name, "Expected to find veth0")
}

func TestStartAndCallback(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

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

	w := &Watcher{
		current:     cache,
		l:           log.Logger().Named(watcherName),
		p:           pubsub.New(),
		refreshRate: 5 * time.Second,
	}

	// When cache only has 1 veth.
	assert.Len(t, w.current, 1, "Expected to find 1 veths")

	// Start watcher manager
	var err error
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err = w.Start(ctx)
		return err
	})
	// Sleep 10 seconds for the watcher to refresh the cache.
	time.Sleep(10 * time.Second)
	assert.NoError(t, err, "Expected no error when refreshing veth cache")
	assert.Len(t, w.current, 2, "Expected to find 2 veths")
	assert.Equal(t, "veth0", w.current[key{
		name:         "veth0",
		hardwareAddr: "00:00:00:00:00:00",
		netNsID:      0,
	}].(netlink.LinkAttrs).Name, "Expected to find veth0")
	assert.Equal(t, "veth1", w.current[key{
		name:         "veth1",
		hardwareAddr: "00:00:00:00:00:01",
		netNsID:      1,
	}].(netlink.LinkAttrs).Name, "Expected to find veth1")
}

func TestStartError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	showLink = func() ([]netlink.Link, error) {
		return nil, errors.New("error")
	}

	w := &Watcher{
		current:     make(cache),
		l:           log.Logger().Named(watcherName),
		p:           pubsub.New(),
		refreshRate: 5 * time.Second,
	}

	err := w.Start(ctx)
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
