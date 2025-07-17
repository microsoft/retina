package ebpfwindows

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/microsoft/retina/pkg/log"
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

type MetricsKey struct {
	Version        uint8
	Reason         uint8
	Dir            uint8
	ExtendedReason uint16
}

type MetricsValue struct {
	Count uint64
	Bytes uint64
}

// IterateCallback represents the signature of the callback function expected by
// the IterateWithCallback method, which in turn is used to iterate all the
// keys/values of a metrics map.
type IterateCallback func(*MetricsKey, *MetricsValue)

// MetricsMap interface represents a metrics map, and can be reused to implement
// mock maps for unit tests.
type MetricsMap interface {
	IterateWithCallback(*log.ZapLogger, IterateCallback) error
}

type metricsMap struct{}

var (
	// Load the retinaebpfapi.dll
	retinaEbpfAPI = windows.NewLazyDLL("retinaebpfapi.dll")
	// Load the RetinaEnumerateMetrics function
	enumMetricsMap = retinaEbpfAPI.NewProc("RetinaEnumerateMetrics")
	// Load the RetinaGetLostEventsCount function
	lostEventCount = retinaEbpfAPI.NewProc("RetinaGetLostEventsCount")
)

// ringBufferEventCallback type definition in Go
type enumMetricsCallback = func(key, value unsafe.Pointer) int

// Callbacks in Go can only be passed as functions with specific signatures and often need to be wrapped in a syscall-compatible function.
var enumCallBack enumMetricsCallback

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

var callEnumMetricsMap = func(callback uintptr) (uintptr, uintptr, error) {
	return enumMetricsMap.Call(callback)
}

// IterateWithCallback iterates through all the keys/values of a metrics map,
// passing each key/value pair to the cb callback
func (m metricsMap) IterateWithCallback(l *log.ZapLogger, cb IterateCallback) error {
	// Define the callback function in Go
	enumCallBack = func(key unsafe.Pointer, value unsafe.Pointer) int {
		if key == nil {
			l.Error("MetricsKey is nil")
			return 1
		}

		if value == nil {
			l.Error("Metrics Value is nil")
			return 1
		}

		metricsValue := (*MetricsValue)(value)
		metricsKey := (*MetricsKey)(key)
		cb(metricsKey, metricsValue)
		return 0
	}

	// Convert the Go function into a syscall-compatible function
	callback := syscall.NewCallback(enumMetricsSysCallCallback)

	// Call the API
	ret, _, err := callEnumMetricsMap(uintptr(callback))

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
	return fmt.Sprintf("Direction: %s, Reason: %s", k.Direction(), k.DropForwardReason())
}

// DropForwardReason gets the forwarded/dropped reason in human readable string format
func (k *MetricsKey) DropForwardReason() string {
	if k.Reason == DropPacketMonitor {
		return k.DropPacketMonitorReason()
	}
	return DropReason(k.Reason)
}

// DropPacketMonitorReason gets the Packer Monitor dropped reason in human readable string format
func (k *MetricsKey) DropPacketMonitorReason() string {
	if k.Reason == DropPacketMonitor {
		return DropReasonExt(k.Reason, uint32(k.ExtendedReason))
	}
	panic("The reason is not DropPacketMonitor")
}

// IsDrop checks if the reason is drop or not.
func (k *MetricsKey) IsDrop() bool {
	return k.Reason == DropInvalid || k.Reason >= DropMin
}

// IsIngress checks if the direction is ingress or not.
func (k *MetricsKey) IsIngress() bool {
	return k.Dir == dirIngress
}

// IsEgress checks if the direction is egress or not.
func (k *MetricsKey) IsEgress() bool {
	return k.Dir == dirEgress
}

func GetLostEventsCount() (uint64, error) {
	ret, _, err := lostEventCount.Call()
	if err != nil && err != syscall.Errno(0) {
		return 0, fmt.Errorf("RetinaGetLostEventsCount call failed: %w", err)
	}
	return uint64(ret), nil
}
