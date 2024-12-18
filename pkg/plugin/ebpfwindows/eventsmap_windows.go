package ebpfwindows

import (
	"syscall"
	"unsafe"
)

var (
	registerEventsMapCallback   = retinaEbpfApi.NewProc("register_cilium_eventsmap_callback")
	unregisterEventsMapCallback = retinaEbpfApi.NewProc("unregister_cilium_eventsmap_callback")
)

type eventsMapCallback func(data unsafe.Pointer, size uint64) int

// Callbacks in Go can only be passed as functions with specific signatures and often need to be wrapped in a syscall-compatible function.
var eventsCallback eventsMapCallback = nil

// This function will be passed to the Windows API
func eventsMapSysCallCallback(data unsafe.Pointer, size uint64) uintptr {

	if eventsCallback != nil {
		return uintptr(eventsCallback(data, size))
	}

	return 0
}

// EventsMap interface represents a events map
type EventsMap interface {
	RegisterForCallback(eventsMapCallback) error
	UnregisterForCallback() error
}

type eventsMap struct {
	ringBuffer uintptr
}

// NewEventsMap creates a new metrics map
func NewEventsMap() EventsMap {
	return &eventsMap{ringBuffer: 0}
}

// RegisterForCallback registers a callback function to be called when a new event is added to the events map
func (e *eventsMap) RegisterForCallback(cb eventsMapCallback) error {

	eventsCallback = cb

	// Convert the Go function into a syscall-compatible function
	callback := syscall.NewCallback(eventsMapSysCallCallback)

	// Call the API
	ret, _, err := registerEventsMapCallback.Call(
		uintptr(callback),
		uintptr(unsafe.Pointer(&e.ringBuffer)),
	)

	if ret != 0 {
		return err
	}

	return nil
}

// UnregisterForCallback unregisters the callback function
func (e *eventsMap) UnregisterForCallback() error {

	// Call the API
	ret, _, err := unregisterEventsMapCallback.Call(e.ringBuffer)

	if ret != 0 {
		return err
	}

	return nil
}
