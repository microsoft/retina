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
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	filterManagerRetries = 3
	hostLookupRetries    = 6 // 6 retries for a total of 63 seconds.
)

type ApiServerWatcher struct {
	isRunning         bool
	l                 *log.ZapLogger
	current           cache
	new               cache
	apiServerHostName string
	hostResolver      IHostResolver
	filterManager     fm.IFilterManager
	restConfig        *rest.Config
	client            kclient.Client
}

var a *ApiServerWatcher

// Watcher creates a new ApiServerWatcher instance.
func Watcher() *ApiServerWatcher {
	if a == nil {
		a = &ApiServerWatcher{
			isRunning:    false,
			l:            log.Logger().Named("apiserver-watcher"),
			current:      make(cache),
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

	// Get filter manager.
	if a.filterManager == nil {
		var err error
		a.filterManager, err = fm.Init(filterManagerRetries)
		if err != nil {
			a.l.Error("failed to init filter manager", zap.Error(err))
			return fmt.Errorf("failed to init filter manager: %w", err)
		}
	}

	// Get  kubeconfig.
	if a.restConfig == nil {
		config, err := kcfg.GetConfig()
		if err != nil {
			a.l.Error("failed to get kubeconfig", zap.Error(err))
			return fmt.Errorf("failed to get kubeconfig: %w", err)
		}
		a.restConfig = config
	}

	if a.client == nil {
		c, err := kclient.New(a.restConfig, kclient.Options{})
		if err != nil {
			a.l.Error("failed to create kubernetes client", zap.Error(err))
			return fmt.Errorf("failed to create kubernetes client: %w", err)
		}
		a.client = c
	}

	hostName, err := a.getHostName()
	if err != nil {
		a.l.Error("failed to get host name", zap.Error(err))
		return fmt.Errorf("failed to get host name: %w", err)
	}
	a.apiServerHostName = hostName

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
		a.l.Error("failed to initialize new cache", zap.Error(err))
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
	svcIPs, err := a.ipsFromService(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve ips from kubernetes service: %w", err)
	}

	endpointIPs, err := a.ipsFromEndpoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve ips from kubernetes endpoint: %w", err)
	}

	ips, err := a.resolveIPs(ctx, a.apiServerHostName)
	if err != nil {
		return fmt.Errorf("failed to resolve IPs: %w", err)
	}

	ips = append(ips, endpointIPs...)
	ips = append(ips, svcIPs...)

	// Reset new cache.
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

func (a *ApiServerWatcher) resolveIPs(ctx context.Context, host string) ([]string, error) {
	// perform a DNS lookup for the host URL using the net.DefaultResolver which uses the local resolver.
	// Possible errors  here are:
	// 	- Canceled context: The context was canceled before the lookup completed.
	// 	-DNS server errors ie NXDOMAIN, SERVFAIL.
	// 	- Network errors ie timeout, unreachable DNS server.
	// 	-Other DNS-related errors encapsulated in a DNSError.
	var hostIPs []string
	var err error

	retryFunc := func() error {
		hostIPs, err = a.hostResolver.LookupHost(ctx, host)
		if err != nil {
			return fmt.Errorf("APIServer LookupHost failed: %w", err)
		}
		return nil
	}

	// Retry the lookup for hostIPs in case of failure.
	err = utils.Retry(retryFunc, hostLookupRetries)
	if err != nil {
		return nil, err
	}

	if len(hostIPs) == 0 {
		a.l.Debug("no IPs found for host", zap.String("host", host))
	}

	return hostIPs, nil
}

// ipsFromService retrieves IP addresses from the master service "kubernetes" in the default namespace.
// These IPs are used as a virtual-ip to the kube-apiserver.
func (a *ApiServerWatcher) ipsFromService(ctx context.Context) ([]string, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes",
			Namespace: "default",
		},
	}
	if err := a.client.Get(ctx, kclient.ObjectKeyFromObject(svc), svc); err != nil {
		return nil, fmt.Errorf("retrieving kubernetes service: %w", err)
	}
	return svc.Spec.ClusterIPs, nil
}

// ipsFromEndpoint retrieves IP addresses from the Endpoint resource "kubernetes" in the default namespace.
// These IPs are the addresses for the kube-apiserver.
func (a *ApiServerWatcher) ipsFromEndpoint(ctx context.Context) ([]string, error) {
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes",
			Namespace: "default",
		},
	}
	if err := a.client.Get(ctx, kclient.ObjectKeyFromObject(ep), ep); err != nil {
		return nil, fmt.Errorf("retrieving kubernetes endpoint: %w", err)
	}
	ips := []string{}
	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			ips = append(ips, addr.IP)
		}
	}
	return ips, nil
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

func (a *ApiServerWatcher) getHostName() (string, error) {
	// Parse the host URL.
	hostURL := a.restConfig.Host
	parsedURL, err := url.ParseRequestURI(hostURL)
	if err != nil {
		log.Logger().Error("failed to parse URL", zap.String("url", hostURL), zap.Error(err))
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract the host name from the URL.
	host := strings.TrimPrefix(parsedURL.Host, "www.")
	if colonIndex := strings.IndexByte(host, ':'); colonIndex != -1 {
		host = host[:colonIndex]
	}
	return host, nil
}
