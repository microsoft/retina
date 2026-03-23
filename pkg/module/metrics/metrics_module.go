// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/crd/api/v1alpha1/validations"
	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	forward       string = "forward"
	drop          string = "drop"
	tcp           string = "tcp"
	nodeApiserver string = "node_apiserver"
	dns           string = "dns"
	pktmon        string = "pktmon"

	metricModuleReq filtermanager.Requestor = "metricModule"
	interval        time.Duration           = 1 * time.Second
)

var (
	m    *Module
	once sync.Once
	// moduleReqMetadata is the metadata for the metric module. This metadata is used for namespaces
	moduleReqMetadata filtermanager.RequestMetadata = filtermanager.RequestMetadata{
		RuleID: "namespace",
	}
	// modulePodReqMetadata is the metadata for the metric module pods.
	// and the ips can be removed separately. Removing a namespace should remove pod ips in that namespace, but if the pod
	// was added to the filtermanager separately, then the pod ip should still exist in the filtermap since it has another reference.
	modulePodReqMetadata filtermanager.RequestMetadata = filtermanager.RequestMetadata{
		RuleID: "pod",
	}
)

type Module struct {
	*sync.RWMutex
	// ctx is the parent context
	ctx context.Context

	ctxCancel context.CancelFunc

	// moduleCtx is the context of the metric module
	moduleCtx context.Context

	// l is the logger
	l *log.ZapLogger

	// daemon config
	daemonConfig *kcfg.Config

	// metricConfigs is the list of metric configurations from CRD
	configs []*api.MetricsConfiguration

	// current metrics spec for metrics module
	currentSpec *api.MetricsSpec

	// pubsub is the pubsub client
	pubsub pubsub.PubSubInterface

	// isRunning is the flag to indicate if the metric module is running
	isRunning bool

	// enricher to read events from
	enricher enricher.EnricherInterface

	// wg is the wait group for the metric module
	wg sync.WaitGroup

	// includedNamespaces for metrics
	includedNamespaces map[string]struct{}

	// excludedNamespaces for metrics
	excludedNamespaces map[string]struct{}

	// metrics registry
	registry map[string]AdvMetricsInterface

	// filterManager to add or delete ip address filters
	filterManager filtermanager.IFilterManager

	// cache is the cache of all the objects
	daemonCache cache.CacheInterface

	// dirtyPods is the map of pod IPs to add or delete
	// from filtermanager
	// todo need metadata , not just net.ip (need new struct)
	dirtyPods *common.DirtyCache

	// ipMetadataTracking tracks which metadata type(s) were used when adding each IP
	// Key: IP address string, Value: metadata tracking info
	// This ensures DELETE uses the same metadata as ADD
	ipMetadataTracking map[string]metadataTrackingInfo

	// pubsub subscription uuid
	pubsubPodSub string
}

func InitModule(ctx context.Context,
	conf *kcfg.Config,
	pubsub pubsub.PubSubInterface,
	enricher enricher.EnricherInterface,
	fm filtermanager.IFilterManager,
	cache cache.CacheInterface,
) *Module {
	// this is a thread-safe singleton instance of the metric module
	once.Do(func() {
		m = &Module{
			RWMutex:            &sync.RWMutex{},
			l:                  log.Logger().Named(string("MetricModule")),
			pubsub:             pubsub,
			configs:            make([]*api.MetricsConfiguration, 0),
			enricher:           enricher,
			wg:                 sync.WaitGroup{},
			registry:           make(map[string]AdvMetricsInterface),
			moduleCtx:          ctx,
			filterManager:      fm,
			daemonCache:        cache,
			dirtyPods:          common.NewDirtyCache(),
			ipMetadataTracking: make(map[string]metadataTrackingInfo),
			pubsubPodSub:       "",
			daemonConfig:       conf,
		}
	})

	return m
}

func (m *Module) Reconcile(spec *api.MetricsSpec) error {
	// If the new spec has not changed, then do nothing.
	if m.currentSpec != nil && m.currentSpec.Equals(spec) {
		m.l.Debug("Spec has not changed. Not reconciling.")
		return nil
	}

	if m.isRunning {
		m.l.Warn("Metric module is running. Cannot reconcile.")
		// need to cancel the current context and create a new one

		m.ctxCancel()
		m.wg.Wait()
	}

	m.l.Info("Reconciling metric module", zap.Any("spec", spec))

	m.Lock()
	defer m.Unlock()

	m.updateNamespaceLists(spec)

	if m.currentSpec == nil || !validations.MetricsContextOptionsCompare(m.currentSpec.ContextOptions, spec.ContextOptions) {
		m.updateMetricsContexts(spec)
	}

	m.currentSpec = spec

	newCtx, cancel := context.WithCancel(m.moduleCtx)
	m.ctxCancel = cancel
	m.ctx = newCtx
	m.run(newCtx)
	return nil
}

func (m *Module) updateNamespaceLists(spec *api.MetricsSpec) {
	if len(spec.Namespaces.Include) > 0 && len(spec.Namespaces.Exclude) > 0 {
		m.l.Error("Both included and excluded namespaces are specified. Cannot reconcile.")
		return
	}

	// Handle include list
	if len(spec.Namespaces.Include) > 0 {
		m.l.Info("Including namespaces", zap.Strings("namespaces", spec.Namespaces.Include))
		m.appendIncludeList(spec.Namespaces.Include)
		m.appendExcludeList([]string{}) // Clear exclude list when using include
	} else if len(spec.Namespaces.Exclude) > 0 {
		// Handle exclude list
		m.l.Info("Excluding namespaces", zap.Strings("namespaces", spec.Namespaces.Exclude))
		m.appendExcludeList(spec.Namespaces.Exclude)
		m.appendIncludeList([]string{}) // Clear include list when using exclude
	} else {
		// Neither include nor exclude specified - clear both
		m.l.Info("No namespace filtering specified - clearing both include and exclude lists")
		m.appendIncludeList([]string{})
		m.appendExcludeList([]string{})
	}

}

func (m *Module) updateMetricsContexts(spec *api.MetricsSpec) {
	// clean old metrics from registry (remove prometheus collectors and remove map entry)
	// reset the advanced metrics registry
	for key, metricObj := range m.registry {
		metricObj.Clean()
		delete(m.registry, key)
	}

	exporter.ResetAdvancedMetricsRegistry()

	ctxType := remoteContext
	if m.daemonConfig != nil && !m.daemonConfig.RemoteContext {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		// so we can getaway with just one context type. For this reason we will only use srccontext
		ctxType = localContext
	}

	for _, ctxOption := range spec.ContextOptions {
		var ttl time.Duration
		var err error
		if ctxOption.TTL != "" {
			ttl, err = time.ParseDuration(ctxOption.TTL)
			// this shouldn't happen since we've already validated the CRD, but put some safety here just in case
			if err != nil {
				m.l.Error("Invalid TTL format", zap.String("metricName", ctxOption.MetricName), zap.Error(err))
				continue
			}
		}
		switch {
		case strings.Contains(ctxOption.MetricName, forward):
			fm := NewForwardCountMetrics(&ctxOption, m.l, ctxType, ttl)
			if fm != nil {
				m.registry[ctxOption.MetricName] = fm
			}
		case strings.Contains(ctxOption.MetricName, drop):
			dm := NewDropCountMetrics(&ctxOption, m.l, ctxType, ttl)
			if dm != nil {
				m.registry[ctxOption.MetricName] = dm
			}
		case strings.Contains(ctxOption.MetricName, tcp):
			tm := NewTCPMetrics(&ctxOption, m.l, ctxType, ttl)
			if tm != nil {
				m.registry[ctxOption.MetricName] = tm
			}
			tr := NewTCPRetransMetrics(&ctxOption, m.l, ctxType, ttl)
			if tr != nil {
				m.registry[ctxOption.MetricName] = tr
			}
		case strings.Contains(ctxOption.MetricName, nodeApiserver):
			// Uses the pattern we will follow in future where each base metric has one instance.
			// Example - tcp, latency, dns, etc.
			lm := NewLatencyMetrics(&ctxOption, m.l, ctxType)
			if lm != nil {
				m.registry[nodeApiserver] = lm
			}
		case strings.Contains(ctxOption.MetricName, dns) || strings.Contains(ctxOption.MetricName, pktmon):
			dm := NewDNSMetrics(&ctxOption, m.l, ctxType, ttl)
			if dm != nil {
				m.registry[ctxOption.MetricName] = dm
			}
		default:
			m.l.Error("Invalid metric name", zap.String("metricName", ctxOption.MetricName))
		}
	}

	for metricName, metricObj := range m.registry {
		metricObj.Init(metricName)
	}
}

func (m *Module) run(newCtx context.Context) {
	if m.isRunning {
		m.l.Warn("Metric module is already running. Cannot start again.")
		return
	}

	cbFunc := pubsub.CallBackFunc(m.PodCallBackFn)
	m.pubsubPodSub = m.pubsub.Subscribe(common.PubSubPods, &cbFunc)

	m.wg.Add(1)
	go func() {
		m.Lock()
		m.isRunning = true
		m.ctx = newCtx
		m.Unlock()

		evReader := m.enricher.ExportReader()
		for {
			ev := evReader.NextFollow(newCtx)
			if ev == nil {
				break
			}

			switch ev.Event.(type) {
			case *flow.Flow:
				m.RLock()
				f := ev.Event.(*flow.Flow)
				m.l.Debug("converted flow object", zap.Any("flow l4", f.IP))
				for _, metricObj := range m.registry {
					metricObj.ProcessFlow(f)
				}
				m.RUnlock()
			case *flow.LostEvent:
				ev := ev.Event.(*flow.LostEvent)
				// the number of lost events == the size of the ring buffer initialized.
				metrics.LostEventsCounter.WithLabelValues(utils.EnricherRing, string(metricModuleReq)).Add(float64(ev.NumEventsLost))
			default:
				m.l.Warn("Unknown event type", zap.Any("event", ev))
			}
		}

		err := evReader.Close()
		if err != nil {
			m.l.Error("Error closing the event reader", zap.Error(err))
		}
		m.Lock()
		m.isRunning = false
		m.ctx = nil
		m.Unlock()

		m.wg.Done()
	}()

	m.wg.Add(1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.l.Debug("Processing dirty pods")
				m.applyDirtyPods()
			case <-newCtx.Done():
				m.l.Info("Context cancelled. Exiting.")
				err := m.pubsub.Unsubscribe(common.PubSubPods, m.pubsubPodSub)
				if err != nil {
					m.l.Error("Error unsubscribing from pubsub", zap.Error(err))
				}
				m.wg.Done()
				return
			}
		}
	}()
}

func (m *Module) appendIncludeList(namespaces []string) {
	if m.includedNamespaces == nil {
		m.includedNamespaces = make(map[string]struct{})
	}

	m.l.Info("Appending namespaces to include list", zap.Strings("namespaces", namespaces))

	// TODO here we will need to check for IP which
	// needs to be added to filter manager and which needs to be removed
	// this logic is for temporary testing purposes. We will need to support multiple scenarios
	// 1. Adding Ips initially when CRD is added
	// 2. adding or deleting IPs when pods get created or deleted in namespace of interest
	// 3. adding or deleting IPs when namespace is added or deleted
	// 4. deleting ips when CRD is deleted

	tempNewNs := make(map[string]struct{})
	for _, ns := range namespaces {
		tempNewNs[ns] = struct{}{}
	}

	m.l.Info("Current included namespaces", zap.Any("namespaces", m.includedNamespaces))
	toAdd, toRemove := make([]string, 0), make([]string, 0)
	for _, ns := range namespaces {
		if _, ok := m.includedNamespaces[ns]; !ok {
			toAdd = append(toAdd, ns)
		}
	}

	for ns := range m.includedNamespaces {
		if _, ok := tempNewNs[ns]; !ok {
			toRemove = append(toRemove, ns)
			delete(m.includedNamespaces, ns)
		}
	}

	if len(m.includedNamespaces) != len(tempNewNs) {
		m.includedNamespaces = tempNewNs
	}

	m.l.Info("Namespaces to add", zap.Strings("namespaces", toAdd))
	m.l.Info("Namespaces to remove", zap.Strings("namespaces", toRemove))

	// toAdd namespace IPs to filter manager
	for _, ns := range toAdd {
		ips := m.daemonCache.GetIPsByNamespace(ns)
		m.l.Info("Adding IPs to filter manager", zap.String("namespace", ns), zap.Any("ips", ips))

		err := m.filterManager.AddIPs(ips, metricModuleReq, moduleReqMetadata)
		if err != nil {
			m.l.Error("Error adding IPs to filter manager", zap.Error(err))
		}
	}

	// toRemove namespace IPs from filter manager
	for _, ns := range toRemove {
		ips := m.daemonCache.GetIPsByNamespace(ns)
		m.l.Info("Removing IPs from filter manager", zap.String("namespace", ns), zap.Any("ips", ips))

		err := m.filterManager.DeleteIPs(ips, metricModuleReq, moduleReqMetadata)
		if err != nil {
			m.l.Error("Error removing IPs from filter manager", zap.Error(err))
		}
	}
}

func (m *Module) appendExcludeList(namespaces []string) {
	if m.excludedNamespaces == nil {
		m.excludedNamespaces = make(map[string]struct{})
	}

	m.l.Info("Appending namespaces to exclude list", zap.Strings("namespaces", namespaces))

	// Build new excluded namespaces map
	tempNewNs := make(map[string]struct{})
	for _, ns := range namespaces {
		tempNewNs[ns] = struct{}{}
	}

	// Track if this is initial setup
	isInitialSetup := len(m.excludedNamespaces) == 0 && len(tempNewNs) > 0

	// Update the excluded namespaces map
	m.excludedNamespaces = tempNewNs

	// Log the exclude list for debugging
	excludeList := make([]string, 0, len(m.excludedNamespaces))
	for ns := range m.excludedNamespaces {
		excludeList = append(excludeList, ns)
	}
	m.l.Info("Current excluded namespaces", zap.Strings("namespaces", excludeList), zap.Int("count", len(m.excludedNamespaces)))

	// For initial setup, add IPs from ALL existing non-excluded namespaces
	if isInitialSetup {
		m.l.Info("Initial exclude list setup - adding IPs from all non-excluded namespaces")
		allNamespaces := m.daemonCache.GetAllNamespaces()

		for _, ns := range allNamespaces {
			if _, excluded := m.excludedNamespaces[ns]; !excluded {
				ips := m.daemonCache.GetIPsByNamespace(ns)
				if len(ips) > 0 {
					m.l.Info("Adding IPs to filter manager",
						zap.String("namespace", ns),
						zap.Int("ip_count", len(ips)))
					err := m.filterManager.AddIPs(ips, metricModuleReq, moduleReqMetadata)
					if err != nil {
						m.l.Error("Error adding IPs to filter manager", zap.Error(err))
					}
				}
			}
		}
	}
}

func (m *Module) PodCallBackFn(obj interface{}) {
	event := obj.(*cache.CacheEvent)
	if event == nil {
		return
	}

	pod := event.Obj.(*common.RetinaEndpoint)
	if pod == nil {
		return
	}

	ip, err := pod.PrimaryNetIP()
	if err != nil || ip == nil {
		m.l.Error("Error getting primary net IP", zap.Any("pod obj", pod), zap.Error(err))
		return
	}

	m.Lock()
	nsInterest := m.nsOfInterest(pod.Namespace())
	podInterest := m.podOfInterest(ip, pod.Annotations())

	if !nsInterest && !podInterest {
		m.Unlock()
		return
	}
	m.Unlock()

	handlePodEvent(event, m, pod, ip)
}

func handlePodEvent(event *cache.CacheEvent, m *Module, pod *common.RetinaEndpoint, ip net.IP) {
	if pod.Name() == common.APIServerEndpointName && pod.Namespace() == common.APIServerEndpointName {
		m.l.Debug("Ignoring apiserver endpoint")
		return
	}
	podCacheEntry := DirtyCachePod{
		IP:         ip,
		Annotated:  m.podAnnotated(pod.Annotations()),
		Namespaced: m.nsOfInterest(pod.Namespace()),
	}

	switch event.Type {
	case cache.EventTypePodAdded:
		// Pod is not annotated NOR is it namespaced (in crd or annotated).
		// This means that we have the stale pod ip in the filtermap so we should remove it.
		// This case handles IP reuse - when an IP is reassigned to a pod that's not of interest.
		// FIX: Don't force flags - let the tracking system use the correct metadata from ADD time
		if !podCacheEntry.Annotated && !podCacheEntry.Namespaced {
			m.l.Info("Adding pod IP to DELETE dirty pods cache. Pod not annotated or in namespace of interest.", zap.String("pod name", pod.NamespacedName()))
			ipKey := ip.String()
			// Don't force flags - the DELETE will use tracked metadata
			m.dirtyPods.ToDelete(ipKey, podCacheEntry)
			return
		}
		m.l.Info("Adding pod IP to ADD dirty pods cache", zap.String("pod name", pod.NamespacedName()))
		m.dirtyPods.ToAdd(podCacheEntry.IP.String(), podCacheEntry)
	case cache.EventTypePodDeleted:
		// Verify the pod is actually gone from the cache before deleting from filter
		// This prevents spurious DELETE events during startup from removing valid pods
		if endpoint := m.daemonCache.GetPodByIP(ip.String()); endpoint != nil {
			m.l.Info("Ignoring DELETE event - pod still exists in cache",
				zap.String("pod", pod.NamespacedName()),
				zap.String("ip", ip.String()))
			return
		}

		ipKey := ip.String()
		m.l.Info("Adding pod IP to DELETE dirty pods cache", zap.String("pod name", pod.NamespacedName()))
		m.dirtyPods.ToDelete(ipKey, podCacheEntry)
	default:
		m.l.Warn("Unknown cache event type", zap.Any("event", event))
		return
	}
}

// Adds or removes pod ips from filtermanager
func (m *Module) applyDirtyPods() {
	m.Lock()
	defer m.Unlock()

	m.applyDirtyPodsAdd()
	m.applyDirtyPodsDelete()
}

// applyDirtyPodsAdd adds pod ips to filtermanager
// if the pod is annotated, then it should be added to the filtermanager with pod request metadata
// if the pod is a namespace of interest, then it should be added to the filtermanager with default request metadata
// there can be overlap here, since if a pod is annotated, and the namespace is annotated, we do not want to remove
// the pod ip from the filtermanager if only one of the annotations is removed.
func (m *Module) applyDirtyPodsAdd() {
	adds := m.dirtyPods.GetAddList()
	if len(adds) > 0 {
		podsToAdd := make([]net.IP, 0)
		podsToAddNamespaced := make([]net.IP, 0)
		for _, entry := range adds {
			podEntry := entry.(DirtyCachePod)
			if podEntry.Annotated {
				podsToAdd = append(podsToAdd, podEntry.IP)
			}
			if podEntry.Namespaced {
				podsToAddNamespaced = append(podsToAddNamespaced, podEntry.IP)
			}
		}
		if len(podsToAdd) > 0 {
			m.l.Debug("Adding annotated pod IPs to filtermap", zap.Any("IPs", podsToAdd))
			err := m.filterManager.AddIPs(podsToAdd, metricModuleReq, modulePodReqMetadata)
			if err != nil {
				m.l.Error("Error adding pod IP to filter manager", zap.Error(err))
			} else {
				// Track that these IPs were added with pod metadata
				for _, ip := range podsToAdd {
					ipKey := ip.String()
					tracking := m.ipMetadataTracking[ipKey]
					tracking.addedWithPodMetadata = true
					m.ipMetadataTracking[ipKey] = tracking
				}
			}
		}
		if len(podsToAddNamespaced) > 0 {
			m.l.Debug("Adding namespaced pod IPs to filtermap", zap.Any("IPs", podsToAddNamespaced))
			err := m.filterManager.AddIPs(podsToAddNamespaced, metricModuleReq, moduleReqMetadata)
			if err != nil {
				m.l.Error("Error adding pod IP to filter manager", zap.Error(err))
			} else {
				// Track that these IPs were added with namespace metadata
				for _, ip := range podsToAddNamespaced {
					ipKey := ip.String()
					tracking := m.ipMetadataTracking[ipKey]
					tracking.addedWithNamespaceMetadata = true
					m.ipMetadataTracking[ipKey] = tracking
				}
			}
		}
	}
	m.dirtyPods.ClearAdd()
}

// applyDirtyPodsDelete deletes pod ips from filtermanager
func (m *Module) applyDirtyPodsDelete() {
	deletes := m.dirtyPods.GetDeleteList()
	if len(deletes) > 0 {
		podOfInterestDeleteList := make([]net.IP, 0)
		namespaceOfInterestDeleteList := make([]net.IP, 0)
		for _, entry := range deletes {
			podEntry := entry.(DirtyCachePod)
			ipKey := podEntry.IP.String()

			// Use tracked metadata from ADD time, not current flags
			tracking, exists := m.ipMetadataTracking[ipKey]

			if !exists {
				m.l.Warn("No metadata tracking info found for IP, using current flags",
					zap.String("ip", ipKey))
				// Fallback to current flags if no tracking info
				if podEntry.Annotated {
					podOfInterestDeleteList = append(podOfInterestDeleteList, podEntry.IP)
				}
				if podEntry.Namespaced {
					namespaceOfInterestDeleteList = append(namespaceOfInterestDeleteList, podEntry.IP)
				}
			} else {
				// Use tracked metadata from ADD time
				if tracking.addedWithPodMetadata {
					podOfInterestDeleteList = append(podOfInterestDeleteList, podEntry.IP)
				}
				if tracking.addedWithNamespaceMetadata {
					namespaceOfInterestDeleteList = append(namespaceOfInterestDeleteList, podEntry.IP)
				}
			}
		}

		if len(podOfInterestDeleteList) > 0 {
			m.l.Debug("Deleting Ips in dirty pods from filtermap", zap.Any("IPs", podOfInterestDeleteList))
			err := m.filterManager.DeleteIPs(podOfInterestDeleteList, metricModuleReq, modulePodReqMetadata)
			if err != nil {
				m.l.Error("Error deleting pod IP from filter manager", zap.Error(err))
			} else {
				// Clear pod metadata tracking after successful delete
				for _, ip := range podOfInterestDeleteList {
					ipKey := ip.String()
					tracking := m.ipMetadataTracking[ipKey]
					tracking.addedWithPodMetadata = false
					m.ipMetadataTracking[ipKey] = tracking
				}
			}
		}
		if len(namespaceOfInterestDeleteList) > 0 {
			m.l.Debug("Deleting Ips in dirty pods from filtermap", zap.Any("IPs", namespaceOfInterestDeleteList))
			err := m.filterManager.DeleteIPs(namespaceOfInterestDeleteList, metricModuleReq, moduleReqMetadata)
			if err != nil {
				m.l.Error("Error deleting pod IP from filter manager", zap.Error(err))
			} else {
				// Clear namespace metadata tracking after successful delete
				for _, ip := range namespaceOfInterestDeleteList {
					ipKey := ip.String()
					tracking := m.ipMetadataTracking[ipKey]
					tracking.addedWithNamespaceMetadata = false
					m.ipMetadataTracking[ipKey] = tracking
				}
			}
		}

		// Clean up tracking map for IPs that have no metadata left
		for _, entry := range deletes {
			ipKey := entry.(DirtyCachePod).IP.String()
			if tracking, exists := m.ipMetadataTracking[ipKey]; exists {
				if !tracking.addedWithPodMetadata && !tracking.addedWithNamespaceMetadata {
					delete(m.ipMetadataTracking, ipKey)
				}
			}
		}
	}

	m.dirtyPods.ClearDelete()
}

// nsOfInterest checks if the namespace is in the included or excluded list
// included namespaces can be defined by crd or automatically applied by annotated namespaces.
func (m *Module) nsOfInterest(ns string) bool {
	if len(m.includedNamespaces) > 0 {
		_, ok := m.includedNamespaces[ns]
		return ok
	} else if len(m.excludedNamespaces) > 0 {
		_, ok := m.excludedNamespaces[ns]
		return !ok
	}
	// No filtering specified - track everything by default
	return true
}

func (m *Module) podAnnotated(annotations map[string]string) bool {
	if m.daemonConfig == nil || !m.daemonConfig.EnableAnnotations {
		return false
	}

	if len(annotations) == 0 {
		return false
	}

	if val, ok := annotations[common.RetinaPodAnnotation]; ok && val == common.RetinaPodAnnotationValue {
		m.l.Debug("Pod is annotated with retina observe annotation", zap.Any("annotations", annotations))
		return true
	}

	return false
}

func (m *Module) podOfInterest(ip net.IP, annotations map[string]string) bool {
	fmHasIP := m.filterManager.HasIP(ip)
	return fmHasIP || m.podAnnotated(annotations)
}
