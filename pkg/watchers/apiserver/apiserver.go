// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	fm "github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func (w *Watcher) Name() string {
	return watcherName
}

// Start the apiserver watcher.
func (w *Watcher) Start(ctx context.Context) error {
	if w.filtermanager == nil {
		w.filtermanager = getFilterManager()
	}
	ticker := time.NewTicker(w.refreshRate)
	for {
		select {
		case <-ctx.Done():
			w.l.Info("context done, stopping apiserver watcher")
			return nil
		case <-ticker.C:
			err := w.initNewCache(ctx)
			if err != nil {
				return err
			}
			// Compare the new ips with the old ones.
			created, deleted := w.diffCache()

			// Publish the new ips.
			createdIps := []net.IP{}
			deletedIps := []net.IP{}

			for _, v := range created {
				w.l.Info("New Apiserver ips:", zap.Any("ip", v))
				ip := net.ParseIP(v.(string)).To4()
				createdIps = append(createdIps, ip)
			}

			for _, v := range deleted {
				w.l.Info("Deleted Apiserver ips:", zap.Any("ip", v))
				ip := net.ParseIP(v.(string)).To4()
				deletedIps = append(deletedIps, ip)
			}

			if len(createdIps) > 0 {
				// Publish the new ips.
				w.publish(createdIps, cc.EventTypeAddAPIServerIPs)
				// Add ips to filter manager if any.
				err := w.filtermanager.AddIPs(createdIps, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
				if err != nil {
					w.l.Error("Failed to add ips to filter manager", zap.Error(err))
				}
			}

			if len(deletedIps) > 0 {
				// Publish the deleted ips.
				w.publish(deletedIps, cc.EventTypeDeleteAPIServerIPs)
				// Delete ips from filter manager if any.
				err := w.filtermanager.DeleteIPs(deletedIps, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
				if err != nil {
					w.l.Error("Failed to delete ips from filter manager", zap.Error(err))
				}
			}

			// update the current cache and reset the new cache
			w.current = w.new.deepcopy()
			w.new = nil
		}
	}
}

// Stop the apiserver watcher.
func (w *Watcher) Stop(_ context.Context) error {
	w.l.Info("stopping apiserver watcher")
	return nil
}

func (w *Watcher) initNewCache(ctx context.Context) error {
	ips, err := w.getApiServerIPs(ctx)
	if err != nil {
		return err
	}

	// Reset the new cache.
	w.new = make(cache)
	for _, ip := range ips {
		w.new[ip] = struct{}{}
	}
	return nil
}

func (w *Watcher) diffCache() (created, deleted []interface{}) {
	// check if there are new ips
	for k := range w.new {
		if _, ok := w.current[k]; !ok {
			created = append(created, k)
		}
	}

	// check if there are deleted ips
	for k := range w.current {
		if _, ok := w.new[k]; !ok {
			deleted = append(deleted, k)
		}
	}
	return
}

func (w *Watcher) getApiServerIPs(ctx context.Context) ([]string, error) {
	// Parse the URL
	host, err := w.retrieveAPIServerHostname()
	if err != nil {
		return nil, err
	}

	// Get the ips for the host
	ips, err := w.resolveIPs(ctx, host)
	if err != nil {
		return nil, err
	}

	return ips, nil
}

// parse url to extract hostname
func (w *Watcher) retrieveAPIServerHostname() (string, error) {
	// Parse the URL
	url, err := url.Parse(w.apiServerURL)
	if err != nil {
		fmt.Println("Failed to parse URL:", err)
		return "", err
	}

	// Remove the scheme (http:// or https://) and port from the host
	host := strings.TrimPrefix(url.Host, "www.")
	colonIndex := strings.IndexByte(host, ':')
	if colonIndex != -1 {
		host = host[:colonIndex]
	}
	return host, nil
}

// Resolve the list of ips for the given host
func (w *Watcher) resolveIPs(ctx context.Context, host string) ([]string, error) {
	hostIps, err := w.hostResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	if len(hostIps) == 0 {
		w.l.Error("no ips found for host", zap.String("host", host))
		return nil, fmt.Errorf("no ips found for host %s", host)
	}

	return hostIps, nil
}

func (w *Watcher) publish(netIPs []net.IP, eventType cc.EventType) {
	if len(netIPs) == 0 {
		return
	}

	ipsToPublish := []string{}
	for _, ip := range netIPs {
		ipsToPublish = append(ipsToPublish, ip.String())
	}
	ps := pubsub.New()
	ps.Publish(common.PubSubAPIServer,
		cc.NewCacheEvent(
			eventType,
			common.NewAPIServerObject(ipsToPublish),
		),
	)
	w.l.Debug("Published event", zap.Any("eventType", eventType), zap.Any("netIPs", ipsToPublish))
}

// getHostURL returns the host url from the config.
func getHostURL() string {
	config, err := kcfg.GetConfig()
	if err != nil {
		log.Logger().Error("failed to get config", zap.Error(err))
		return ""
	}
	return config.Host
}

// Get FilterManager
func getFilterManager() *fm.FilterManager {
	f, err := fm.Init(filterManagerRetries)
	if err != nil {
		w.l.Error("failed to init filter manager", zap.Error(err))
	}
	return f
}
