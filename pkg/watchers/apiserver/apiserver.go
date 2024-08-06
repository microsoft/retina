// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package apiserver

import (
	"context"
	"errors"
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
	apiServerURL  string
	hostResolver  IHostResolver
	filterManager fm.IFilterManager
}

var a *ApiServerWatcher

// Watcher creates a new ApiServerWatcher instance.
func Watcher() *ApiServerWatcher {
	if a == nil {
		a = &ApiServerWatcher{
			isRunning:    false,
			l:            log.Logger().Named("apiserver-watcher"),
			current:      make(cache),
			apiServerURL: getHostURL(),
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

	a.filterManager = a.getFilterManager()
	if a.filterManager == nil {
		return errors.New("failed to initialize filter manager")
	}
	a.isRunning = true
	return nil
}

// Stop stops the ApiServerWatcher.
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
	// Compare the new IPs with the old ones.
	created, deleted := a.diffCache()

	createdIPs := []net.IP{}
	deletedIPs := []net.IP{}

	for _, v := range created {
		a.l.Info("New Apiserver IPs:", zap.Any("ip", v))
		ip := net.ParseIP(v.(string)).To4()
		createdIPs = append(createdIPs, ip)
	}

	for _, v := range deleted {
		a.l.Info("Deleted Apiserver IPs:", zap.Any("ip", v))
		ip := net.ParseIP(v.(string)).To4()
		deletedIPs = append(deletedIPs, ip)
	}

	if len(createdIPs) > 0 {
		a.publish(createdIPs, cc.EventTypeAddAPIServerIPs)
		err := a.filterManager.AddIPs(createdIPs, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
		if err != nil {
			a.l.Error("Failed to add IPs to filter manager", zap.Error(err))
		}
	}

	if len(deletedIPs) > 0 {
		a.publish(deletedIPs, cc.EventTypeDeleteAPIServerIPs)
		err := a.filterManager.DeleteIPs(deletedIPs, "apiserver-watcher", fm.RequestMetadata{RuleID: "apiserver-watcher"})
		if err != nil {
			a.l.Error("Failed to delete IPs from filter manager", zap.Error(err))
		}
	}

	a.current = a.new.deepcopy()
	a.new = nil

	return nil
}

func (a *ApiServerWatcher) initNewCache(ctx context.Context) error {
	ips := a.getApiServerIPs(ctx)
	a.new = make(cache)
	for _, ip := range ips {
		a.new[ip] = struct{}{}
	}
	return nil
}

func (a *ApiServerWatcher) diffCache() (created, deleted []interface{}) {
	// Check if there are any new IPs.
	for k := range a.new {
		if _, ok := a.current[k]; !ok {
			created = append(created, k)
		}
	}

	// Check if there are any deleted IPs.
	for k := range a.current {
		if _, ok := a.new[k]; !ok {
			deleted = append(deleted, k)
		}
	}
	return
}

func (a *ApiServerWatcher) getApiServerIPs(ctx context.Context) []string {
	host := a.retrieveApiServerHostname()
	ips := a.resolveIPs(ctx, host)
	return ips
}

func (a *ApiServerWatcher) retrieveApiServerHostname() string {
	parsedURL, err := url.Parse(a.apiServerURL)
	if err != nil {
		a.l.Warn("failed to parse URL", zap.String("url", a.apiServerURL), zap.Error(err))
		return ""
	}

	host := strings.TrimPrefix(parsedURL.Host, "www.")
	if colonIndex := strings.IndexByte(host, ':'); colonIndex != -1 {
		host = host[:colonIndex]
	}
	return host
}

func (a *ApiServerWatcher) resolveIPs(ctx context.Context, host string) []string {
	hostIPs, err := a.hostResolver.LookupHost(ctx, host)
	if err != nil {
		a.l.Warn("failed to resolve IPs for host", zap.String("host", host), zap.Error(err))
		return nil
	}

	if len(hostIPs) == 0 {
		a.l.Warn("no IPs found for host", zap.String("host", host))
		return nil
	}

	return hostIPs
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
	ps.Publish(common.PubSubAPIServer, cc.NewCacheEvent(eventType, common.NewAPIServerObject(ipsToPublish)))
	a.l.Debug("Published event", zap.Any("eventType", eventType), zap.Any("netIPs", ipsToPublish))
}

func getHostURL() string {
	config, err := kcfg.GetConfig()
	if err != nil {
		log.Logger().Error("failed to get config", zap.Error(err))
		return ""
	}
	return config.Host
}

func (a *ApiServerWatcher) getFilterManager() *fm.FilterManager {
	f, err := fm.Init(filterManagerRetries)
	if err != nil {
		a.l.Error("failed to init filter manager", zap.Error(err))
	}
	return f
}
