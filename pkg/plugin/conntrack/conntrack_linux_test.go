package conntrack

import (
	"context"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/bpf"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
)

// TestStartAndStop tests the Start and Stop functions of the conntrack plugin
func TestStartAndStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts()) // nolint:errcheck // ignore
	err := bpf.MountRetinaBpfFS()
	if err != nil {
		t.Errorf("Failed to mount Retina bpf filesystem: %v", err)
	}
	// Initialize the conntrack bpf object
	objs := &conntrackObjects{}
	// Initialize the conntrack map
	ctmap, err := ebpf.NewMapWithOptions(
		&ebpf.MapSpec{
			Name:       "retina_conntrack_map",
			Type:       ebpf.LRUHash,
			KeySize:    16,
			ValueSize:  16,
			MaxEntries: 1024,
			Pinning:    ebpf.PinByName,
		}, ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	)
	if err != nil {
		t.Errorf("Failed to create conntrack map: %v", err)
	}
	objs.conntrackMaps.RetinaConntrackMap = ctmap

	// Create the conntrack plugin
	ct := conntrack{
		l:           log.Logger().Named(string(Name)),
		objs:        objs,
		ctMap:       objs.conntrackMaps.RetinaConntrackMap,
		gcFrequency: 1 * time.Second,
	}
	// Create a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Start the conntrack plugin
	err = ct.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start conntrack plugin: %v", err)
	}
	// Stop the conntrack plugin
	err = ct.Stop()
	if err != nil {
		t.Errorf("Failed to stop conntrack plugin: %v", err)
	}
}

func TestGCForClosingConnection(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts()) // nolint:errcheck // ignore
	err := bpf.MountRetinaBpfFS()
	if err != nil {
		t.Errorf("Failed to mount Retina bpf filesystem: %v", err)
	}
	// Initialize the conntrack bpf object
	objs := &conntrackObjects{}
	ctMap, err := ebpf.NewMapWithOptions(
		&ebpf.MapSpec{
			Name:       plugincommon.ConntrackMapName,
			Type:       ebpf.LRUHash,
			KeySize:    16,
			ValueSize:  16,
			MaxEntries: 1024,
			Pinning:    ebpf.PinByName,
		},
		ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	)
	if err != nil {
		t.Errorf("Failed to create conntrack map: %v", err)
	}
	objs.conntrackMaps.RetinaConntrackMap = ctMap
	// Create the conntrack plugin
	ct := conntrack{
		l:           log.Logger().Named(string(Name)),
		objs:        objs,
		ctMap:       objs.conntrackMaps.RetinaConntrackMap,
		gcFrequency: 1 * time.Second,
	}
	// Populate the conntrack map
	normalKey := conntrackCtV4Key{
		SrcIp:   1,
		DstIp:   2,
		SrcPort: 3,
		DstPort: 4,
		Proto:   5,
	}

	keyToBeDeleted := conntrackCtV4Key{
		SrcIp:   6,
		DstIp:   7,
		SrcPort: 8,
		DstPort: 9,
		Proto:   10,
	}

	normalValue := conntrackCtValue{
		IsClosing: 0,
	}

	valueToBeDeleted := conntrackCtValue{
		IsClosing: 1,
	}

	err = ct.ctMap.Put(normalKey, normalValue)
	if err != nil {
		t.Errorf("Failed to put normal key: %v", err)
	}

	err = ct.ctMap.Put(keyToBeDeleted, valueToBeDeleted)
	if err != nil {
		t.Errorf("Failed to put key to be deleted: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = ct.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start conntrack plugin: %v", err)
	}

	// Use LookUp to check if the key to be deleted is still present
	var lookupValue conntrackCtValue
	err = ct.ctMap.Lookup(keyToBeDeleted, &lookupValue)
	if err == nil {
		t.Errorf("Key to be deleted is still present in the map")
	}

	err = ct.Stop()
	if err != nil {
		t.Errorf("Failed to stop conntrack plugin: %v", err)
	}
}

func TestGCForExpiredConnection(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts()) // nolint:errcheck // ignore
	err := bpf.MountRetinaBpfFS()
	if err != nil {
		t.Errorf("Failed to mount Retina bpf filesystem: %v", err)
	}
	// Initialize the conntrack bpf object
	objs := &conntrackObjects{}
	ctMap, err := ebpf.NewMapWithOptions(
		&ebpf.MapSpec{
			Name:       plugincommon.ConntrackMapName,
			Type:       ebpf.LRUHash,
			KeySize:    16,
			ValueSize:  16,
			MaxEntries: 1024,
			Pinning:    ebpf.PinByName,
		},
		ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	)
	if err != nil {
		t.Errorf("Failed to create conntrack map: %v", err)
	}
	objs.conntrackMaps.RetinaConntrackMap = ctMap
	// Create the conntrack plugin
	ct := conntrack{
		l:           log.Logger().Named(string(Name)),
		objs:        objs,
		ctMap:       objs.conntrackMaps.RetinaConntrackMap,
		gcFrequency: 1 * time.Second,
	}
	// Populate the conntrack map
	normalKey := conntrackCtV4Key{
		SrcIp:   1,
		DstIp:   2,
		SrcPort: 3,
		DstPort: 4,
		Proto:   5,
	}

	keyToBeDeleted := conntrackCtV4Key{
		SrcIp:   6,
		DstIp:   7,
		SrcPort: 8,
		DstPort: 9,
		Proto:   10,
	}

	// Set the monotonic offset to 0
	ktime.MonotonicOffset = 0

	normalValue := conntrackCtValue{
		Lifetime: uint32(time.Now().Add(150 * time.Second).Unix()),
	}

	valueToBeDeleted := conntrackCtValue{
		Lifetime: 0,
	}

	err = ct.ctMap.Put(normalKey, normalValue)
	if err != nil {
		t.Errorf("Failed to put normal key: %v", err)
	}

	err = ct.ctMap.Put(keyToBeDeleted, valueToBeDeleted)
	if err != nil {
		t.Errorf("Failed to put key to be deleted: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = ct.Start(ctx)
	if err != nil {
		t.Errorf("Failed to start conntrack plugin: %v", err)
	}

	// Use LookUp to check if the key to be deleted is still present
	var lookupValue conntrackCtValue
	err = ct.ctMap.Lookup(keyToBeDeleted, &lookupValue)
	if err == nil {
		t.Errorf("Key to be deleted is still present in the map")
	}

	err = ct.Stop()
	if err != nil {
		t.Errorf("Failed to stop conntrack plugin: %v", err)
	}
}
