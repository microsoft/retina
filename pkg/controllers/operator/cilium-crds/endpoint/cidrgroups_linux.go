// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/pkg/errors"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/cilium/pkg/k8s/utils"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

type cidrGroupEntry struct {
	prefixes []netip.Prefix
	lbls     labels.Labels
}

// cidrGroupCache maps CiliumCIDRGroup name to its prefixes and the labels Cilium puts on
// identities of those prefixes. It has its own lock so it can be read while r.Mutex is held.
type cidrGroupCache struct {
	mu     sync.RWMutex
	groups map[string]cidrGroupEntry
}

func newCIDRGroupCache() *cidrGroupCache {
	return &cidrGroupCache{groups: make(map[string]cidrGroupEntry)}
}

// newCIDRGroupEntry mirrors cidrsAndLabelsForCIDRGroup in cilium/pkg/policy/k8s/cilium_cidr_group.go,
// so cidrGroupRef and cidrGroupSelector rules select Windows Pod identities the same way
// they select the agent's prefix identities. Invalid CIDRs are skipped, as in Cilium.
func newCIDRGroupEntry(group *ciliumv2.CiliumCIDRGroup) cidrGroupEntry {
	filtered := utils.RemoveCiliumLabels(group.Labels)
	lbls := make(labels.Labels, len(filtered)*2+1)
	for k, v := range filtered {
		l := labels.NewLabel(k, v, labels.LabelSourceCIDRGroup)
		lbls[l.Key] = l
		encoded := labels.EncodedCIDRGroupLabel(l.Key, l.Value, l.Source)
		lbls[encoded.Key] = encoded
	}
	ref := api.LabelForCIDRGroupRef(group.Name)
	lbls[ref.Key] = ref

	prefixes := make([]netip.Prefix, 0, len(group.Spec.ExternalCIDRs))
	for _, c := range group.Spec.ExternalCIDRs {
		pfx, err := netip.ParsePrefix(string(c))
		if err != nil {
			continue
		}
		prefixes = append(prefixes, pfx.Masked())
	}
	return cidrGroupEntry{prefixes: prefixes, lbls: lbls}
}

func (c *cidrGroupCache) upsert(group *ciliumv2.CiliumCIDRGroup) {
	entry := newCIDRGroupEntry(group)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.groups[group.Name] = entry
}

func (c *cidrGroupCache) delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.groups, name)
}

// contains reports whether the group has a prefix containing any of addrs.
func (c *cidrGroupCache) contains(name string, addrs ...netip.Addr) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.groups[name]
	return ok && prefixesContain(entry.prefixes, addrs...)
}

// labelsFor returns the merged labels of every group with a prefix containing any of addrs.
func (c *cidrGroupCache) labelsFor(addrs ...netip.Addr) labels.Labels {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var lbls labels.Labels
	for _, entry := range c.groups {
		if !prefixesContain(entry.prefixes, addrs...) {
			continue
		}
		if lbls == nil {
			lbls = make(labels.Labels, len(entry.lbls))
		}
		for k, l := range entry.lbls {
			lbls[k] = l
		}
	}
	return lbls
}

func prefixesContain(prefixes []netip.Prefix, addrs ...netip.Addr) bool {
	for _, p := range prefixes {
		for _, addr := range addrs {
			if p.Contains(addr) {
				return true
			}
		}
	}
	return false
}

// cidrGroupLabelsEnabled reports whether Windows Pod identities get CIDR group labels.
func (r *endpointReconciler) cidrGroupLabelsEnabled() bool {
	return r.cfg.WindowsCIDRLabels && r.cidrGroups != nil
}

// runCIDRGroupEvents keeps the CIDR group cache current and relabels affected Pods.
// cidrGroupsSynced is closed once the initial list is cached, so Pods are not labelled
// before their groups are known.
func (r *endpointReconciler) runCIDRGroupEvents(pctx context.Context) error {
	r.l.Info("start to watch CiliumCIDRGroups for Windows Pod labels")

	for ev := range r.cidrGroups.Events(pctx) {
		var err error
		switch ev.Kind {
		case resource.Sync:
			r.cidrGroupsSyncOnce.Do(func() { close(r.cidrGroupsSynced) })
		case resource.Upsert:
			r.cidrGroupCache.upsert(ev.Object)
			err = r.reconcilePodsForCIDRGroup(pctx, ev.Key.Name)
		case resource.Delete:
			r.cidrGroupCache.delete(ev.Key.Name)
			err = r.reconcilePodsForCIDRGroup(pctx, ev.Key.Name)
		}
		if err != nil {
			r.l.Error("failed to relabel pods for CiliumCIDRGroup. requeuing", "error", err, "cidrGroup", ev.Key.Name)
		}
		ev.Done(err)
	}

	r.l.Info("stop watching CiliumCIDRGroups")
	return nil
}

// cidrGroupSyncTimeout bounds the startup wait, e.g. when the CiliumCIDRGroup v2 CRD is missing.
// Pods labelled before the groups sync are relabelled by the group events that follow.
var cidrGroupSyncTimeout = time.Minute

// waitForCIDRGroups waits until the CIDR group cache has its initial contents.
func (r *endpointReconciler) waitForCIDRGroups(ctx context.Context) {
	if !r.cidrGroupLabelsEnabled() {
		return
	}
	select {
	case <-r.cidrGroupsSynced:
		r.l.Info("CiliumCIDRGroups synced")
	case <-time.After(cidrGroupSyncTimeout):
		r.l.Warn("CiliumCIDRGroups not synced, continuing without them for now", "timeout", cidrGroupSyncTimeout)
	case <-ctx.Done():
	}
}

// reconcilePodsForCIDRGroup recomputes labels for Pods that carry the group's label or with an IP
// (IPv4 or IPv6) in the group's current prefixes. Using the stored labels (updated only on success) means a
// retry still finds Pods that left the group even after the cache no longer has the old prefixes.
func (r *endpointReconciler) reconcilePodsForCIDRGroup(pctx context.Context, name string) error {
	r.Lock()
	defer r.Unlock()

	refKey := api.LabelForCIDRGroupRef(name).Key
	var errs []error
	for _, pod := range r.store.ListPods() {
		if pod.toDelete || pod.podObj == nil {
			continue
		}
		_, labelled := pod.lbls[refKey]
		inGroup := r.cidrGroupCache.contains(name, validAddrs(podAddrs(pod.podObj))...)
		if !labelled && !inGroup {
			continue
		}

		ctx, cancel := context.WithTimeout(pctx, RequestTimeout)
		err := r.relabelPod(ctx, pod)
		cancel()
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "pod %s", pod.key.String()))
		}
	}
	if len(errs) > 0 {
		return errors.Errorf("failed to relabel %d pod(s), first error: %v", len(errs), errs[0])
	}
	return nil
}

// relabelPod recomputes a cached Pod's labels and upserts it. Callers must hold r.Mutex.
func (r *endpointReconciler) relabelPod(ctx context.Context, pod *PodEndpoint) error {
	newPEP := pod.deepCopy()
	lbls, err := r.ciliumEndpointsLabels(ctx, pod.podObj)
	if err != nil {
		return errors.Wrap(err, "failed to get pod labels")
	}
	newPEP.lbls = lbls
	r.l.Debug("relabelling pod", "podKey", pod.key.String(), "old labels", pod.lbls, "new labels", newPEP.lbls)
	return r.handlePodUpsert(ctx, newPEP)
}
