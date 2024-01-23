// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	fm "github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	filterManagerRetries = 3
)

type ApiServerWatcher struct {
	isRunning     bool
	l             *log.ZapLogger
	current       cache
	new           cache
	apiServerUrl  string
	hostResolver  IHostResolver
	filtermanager fm.IFilterManager
}

var a *ApiServerWatcher

// NewApiServerWatcher creates a new apiserver watcher.
func Watcher() *ApiServerWatcher {
	if a == nil {
		a = &ApiServerWatcher{
			isRunning:    false,
			l:            log.Logger().Named("apiserver-watcher"),
			current:      make(cache),
			apiServerUrl: getHostURL(),
			hostResolver: net.DefaultResolver,
		}
	}

	return a
}

func (a *ApiServerWatcher) Init(ctx context.Context) error {
	if a.isRunning {
		a.l.Info("apiserver watcher is already running")
		return nil
	}

	a.filtermanager = getFilterManager()
	a.isRunning = true
	return nil
}

// Stop the apiserver watcher.
func (a *ApiServerWatcher) Stop(ctx context.Context) error {
	if !a.isRunning {
		a.l.Info("apiserver watcher is not running")
		return nil
	}
	a.isRunning = false
	return nil
}

func (a *ApiServerWatcher) Refresh(ctx context.Context) error {
	err := a.initNewCache(ctx)
	if err != nil {
		return err
	}
	// Compare the new ips with the old ones.
	created, deleted := a.diffCache()

	// Publish the new ips.
	createdIps := []net.IP{}
	deletedIps := []net.IP{}

	for _, v := range created {
		a.l.Info("New Apiserver ips:", zap.Any("ip", v))
		ip := net.ParseIP(v.(string)).To4()
		createdIps = append(createdIps, ip)
	}

	for _, v := range deleted {
		a.l.Info("Deleted Apiserver ips:", zap.Any("ip", v))
		ip := net.ParseIP(v.(string)).To4()
		deletedIps = append(deletedIps, ip)
	}

	if len(createdIps) > 0 {
		// Publish the new ips.
		a.publish(createdIps, cc.EventTypeAddAPIServerIPs)
		// Add ips to filter manager if any.
		err := a.filtermanager.AddIPs(createdIps, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
		if err != nil {
			a.l.Error("Failed to add ips to filter manager", zap.Error(err))
		}
	}

	if len(deletedIps) > 0 {
		// Publish the deleted ips.
		a.publish(deletedIps, cc.EventTypeDeleteAPIServerIPs)
		// Delete ips from filter manager if any.
		err := a.filtermanager.DeleteIPs(deletedIps, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
		if err != nil {
			a.l.Error("Failed to delete ips from filter manager", zap.Error(err))
		}
	}

	// update the current cache and reset the new cache
	a.current = a.new.deepcopy()
	a.new = nil

	return nil
}

func (a *ApiServerWatcher) initNewCache(ctx context.Context) error {
	ips, err := a.getApiServerIPs(ctx)
	if err != nil {
		return err
	}

	// Reset the new cache.
	a.new = make(cache)
	for _, ip := range ips {
		a.new[ip] = struct{}{}
	}
	return nil
}

func (a *ApiServerWatcher) diffCache() (created, deleted []interface{}) {
	// check if there are new ips
	for k := range a.new {
		if _, ok := a.current[k]; !ok {
			created = append(created, k)
		}
	}

	// check if there are deleted ips
	for k := range a.current {
		if _, ok := a.new[k]; !ok {
			deleted = append(deleted, k)
		}
	}
	return
}

func (a *ApiServerWatcher) getApiServerIPs(ctx context.Context) ([]string, error) {
	// Parse the URL
	host, err := a.retrieveApiServerHostname()
	if err != nil {
		return nil, err
	}

	// Get the ips for the host
	ips, err := a.resolveIPs(ctx, host)
	if err != nil {
		return nil, err
	}

	return ips, nil
}

// parse url to extract hostname
func (a *ApiServerWatcher) retrieveApiServerHostname() (string, error) {
	// Parse the URL
	url, err := url.Parse(a.apiServerUrl)
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
func (a *ApiServerWatcher) resolveIPs(ctx context.Context, host string) ([]string, error) {
	hostIps, err := a.hostResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	if len(hostIps) == 0 {
		a.l.Error("no ips found for host", zap.String("host", host))
		return nil, fmt.Errorf("no ips found for host %s", host)
	}

	return hostIps, nil
}

func (a *ApiServerWatcher) publish(netIPs []net.IP, eventType cc.EventType) {
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
	a.l.Debug("Published event", zap.Any("eventType", eventType), zap.Any("netIPs", ipsToPublish))
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
		a.l.Error("failed to init filter manager", zap.Error(err))
	}
	return f
}
