package ebpfwindows

import (
	"syscall"
	"unsafe"

	"github.com/microsoft/retina/pkg/log"
)

var (
	registerEventsMapCallback   = retinaEbpfAPI.NewProc("RetinaRegisterEventsMapCallback")
	unregisterEventsMapCallback = retinaEbpfAPI.NewProc("RetinaUnregisterEventsMapCallback")
)

type eventsMapCallback func(data unsafe.Pointer, size uint32)

// Callbacks in Go can only be passed as functions with specific signatures and often need to be wrapped in a syscall-compatible function.
var eventsCallback eventsMapCallback

// This function will be passed to the Windows API
func eventsMapSysCallCallback(data unsafe.Pointer, size uint32) int {

	if eventsCallback != nil {
		eventsCallback(data, size)
	}

	return 0
}

// EventsMap interface represents a events map
type EventsMap interface {
	RegisterForCallback(*log.ZapLogger, eventsMapCallback) error
	UnregisterForCallback() error
}

type eventsMap struct {
	perfBuffer uintptr
}

// NewEventsMap creates a new metrics map
func NewEventsMap() EventsMap {
	return &eventsMap{perfBuffer: 0}
}

// RegisterForCallback registers a callback function to be called when a new event is added to the events map
func (e *eventsMap) RegisterForCallback(l *log.ZapLogger, cb eventsMapCallback) error {

	eventsCallback = cb

	l.Info("Attempting to register")
	// Convert the Go function into a syscall-compatible function
	callback := syscall.NewCallback(eventsMapSysCallCallback)

	// Call the API
	ret, _, err := registerEventsMapCallback.Call(
		uintptr(callback),
		uintptr(unsafe.Pointer(&e.perfBuffer)),
	)

	if ret != 0 {
		l.Error("Error registering for events map callback")
		return err
	}

	return nil
}

// UnregisterForCallback unregisters the callback function
func (e *eventsMap) UnregisterForCallback() error {

	// Call the API
	ret, _, err := unregisterEventsMapCallback.Call(e.perfBuffer)

	if ret != 0 {
		return err
	}

	return nil
}
