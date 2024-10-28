package telemetry

import (
	"runtime"
	"strconv"
)

type PerfProfile struct{}

func (p *PerfProfile) GetMemoryUsage() map[string]string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	props := map[string]string{
		heapalloc:  strconv.FormatUint(bToMb(m.HeapAlloc), 10),
		sysmem:     strconv.FormatUint(bToMb(m.Sys), 10),
		goroutines: strconv.Itoa(runtime.NumGoroutine()),
	}
	return props
}

func NewPerfProfile() (*PerfProfile, error) {
	return &PerfProfile{}, nil
}

func (p *PerfProfile) GetCPUUsage() (map[string]string, error) { //nolint unnamed results are fine
	return make(map[string]string), nil
}
