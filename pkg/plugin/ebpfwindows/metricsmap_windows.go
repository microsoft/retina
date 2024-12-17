package ebpfwindows

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	dirUnknown = 0
	dirIngress = 1
	dirEgress  = 2
	dirService = 3
)

// direction is the metrics direction i.e ingress (to an endpoint),
// egress (from an endpoint) or service (NodePort service being accessed from
// outside or a ClusterIP service being accessed from inside the cluster).
// If it's none of the above, we return UNKNOWN direction.
var direction = map[uint8]string{
	dirUnknown: "UNKNOWN",
	dirIngress: "INGRESS",
	dirEgress:  "EGRESS",
	dirService: "SERVICE",
}

// Value must be in sync with struct metrics_key in <bpf/lib/common.h>
type MetricsKey struct {
	Reason uint8 `align:"reason"`
	Dir    uint8 `align:"dir"`
	// Line contains the line number of the metrics statement.
	Line uint16 `align:"line"`
	// File is the number of the source file containing the metrics statement.
	File     uint8    `align:"file"`
	Reserved [3]uint8 `align:"reserved"`
}

// Value must be in sync with struct metrics_value in <bpf/lib/common.h>
type MetricsValue struct {
	Count uint64 `align:"count"`
	Bytes uint64 `align:"bytes"`
}

// MetricsMapValues is a slice of MetricsMapValue
type MetricsValues []MetricsValue

// IterateCallback represents the signature of the callback function expected by
// the IterateWithCallback method, which in turn is used to iterate all the
// keys/values of a metrics map.
type IterateCallback func(*MetricsKey, *MetricsValues)

// MetricsMap interface represents a metrics map, and can be reused to implement
// mock maps for unit tests.
type MetricsMap interface {
	IterateWithCallback(IterateCallback) error
}

type metricsMap struct {
}

var (
	// Load the retinaebpfapi.dll
	retinaEbpfApi = windows.NewLazyDLL("retinaebpfapi.dll")
	// Load the enumerate_cilium_metricsmap function
	enumMetricsMap = retinaEbpfApi.NewProc("enumerate_cilium_metricsmap")
)

// ringBufferEventCallback type definition in Go
type enumMetricsCallback = func(key, value unsafe.Pointer) int

// Callbacks in Go can only be passed as functions with specific signatures and often need to be wrapped in a syscall-compatible function.
var enumCallBack enumMetricsCallback = nil

// This function will be passed to the Windows API
func enumMetricsSysCallCallback(key, value unsafe.Pointer) uintptr {

	if enumCallBack != nil {
		return uintptr(enumCallBack(key, value))
	}

	return 0
}

// NewMetricsMap creates a new metrics map
func NewMetricsMap() MetricsMap {
	return &metricsMap{}
}

// IterateWithCallback iterates through all the keys/values of a metrics map,
// passing each key/value pair to the cb callback
func (m metricsMap) IterateWithCallback(cb IterateCallback) error {

	// Define the callback function in Go
	enumCallBack = func(key unsafe.Pointer, value unsafe.Pointer) int {
		metricsKey := (*MetricsKey)(key)
		metricsValues := (*MetricsValues)(value)
		cb(metricsKey, metricsValues)
		return 0
	}

	// Convert the Go function into a syscall-compatible function
	callback := syscall.NewCallback(enumMetricsSysCallCallback)

	// Call the API
	ret, _, err := enumMetricsMap.Call(
		uintptr(callback),
	)

	if ret != 0 {
		return err
	}

	return nil
}

// MetricDirection gets the direction in human readable string format
func MetricDirection(dir uint8) string {
	if desc, ok := direction[dir]; ok {
		return desc
	}
	return direction[dirUnknown]
}

// Direction gets the direction in human readable string format
func (k *MetricsKey) Direction() string {
	return MetricDirection(k.Dir)
}

// String returns the key in human readable string format
func (k *MetricsKey) String() string {
	return fmt.Sprintf("Direction: %s, Reason: %s, File: %s, Line: %d", k.Direction(), DropReason(k.Reason), BPFFileName(k.File), k.Line)
}

// DropForwardReason gets the forwarded/dropped reason in human readable string format
func (k *MetricsKey) DropForwardReason() string {
	return DropReason(k.Reason)
}

// FileName returns the filename where the event occurred, in string format.
func (k *MetricsKey) FileName() string {
	return BPFFileName(k.File)
}

// IsDrop checks if the reason is drop or not.
func (k *MetricsKey) IsDrop() bool {
	return k.Reason == DropInvalid || k.Reason >= DropMin
}

// Count returns the sum of all the per-CPU count values
func (vs MetricsValues) Count() uint64 {
	c := uint64(0)
	for _, v := range vs {
		c += v.Count
	}

	return c
}

// Bytes returns the sum of all the per-CPU bytes values
func (vs MetricsValues) Bytes() uint64 {
	b := uint64(0)
	for _, v := range vs {
		b += v.Bytes
	}

	return b
}

func (vs MetricsValues) String() string {
	return fmt.Sprintf("Count: %d, Bytes: %d", vs.Count(), vs.Bytes())
}
