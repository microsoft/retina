//go:build unix

package telemetry

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

var (
	microsecondBitShift = 20
	ErrNotInitialized   = errors.New("perf profile not initialized")
)

const (
	userCPUSeconds = "usr_cpu_sec"
	sysCPUSeconds  = "sys_cpu_sec"
)

type PerfProfile struct {
	perflock sync.RWMutex
	usage    *syscall.Rusage
}

func NewPerfProfile() (*PerfProfile, error) {
	p := &PerfProfile{}
	var usage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	if err != nil {
		return nil, fmt.Errorf("failed to get rusage during init: %w", err)
	}
	p.usage = &usage

	return p, nil
}

func (p *PerfProfile) GetMemoryUsage() map[string]string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	props := map[string]string{
		allocatedmem: strconv.FormatUint(bToMb(m.Alloc), 10),
		sysmem:       strconv.FormatUint(bToMb(m.Sys), 10),
		heapallocmem: strconv.FormatUint(bToMb(m.HeapAlloc), 10),
		heapobjects:  strconv.FormatUint(m.HeapObjects, 10),
		heapsys:      strconv.FormatUint(bToMb(m.HeapSys), 10),
		stackinuse:   strconv.FormatUint(m.StackInuse, 10),
		stacksys:     strconv.FormatUint(bToMb(m.StackSys), 10),
		goroutines:   strconv.Itoa(runtime.NumGoroutine()),
	}
	return props
}

func (p *PerfProfile) GetCPUUsage() (map[string]string, error) { //nolint unnamed results are fine
	props := make(map[string]string)
	if p.usage == nil {
		return props, ErrNotInitialized
	}

	p.perflock.Lock()
	defer p.perflock.Unlock()
	var currentUsage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &currentUsage)
	if err != nil {
		return props, fmt.Errorf("failed to get rusage: %w", err)
	}

	userTime := (currentUsage.Utime.Sec - p.usage.Utime.Sec)
	userTime += int64(currentUsage.Utime.Usec-p.usage.Utime.Usec) >> microsecondBitShift

	sysTime := currentUsage.Stime.Sec - p.usage.Stime.Sec
	sysTime += int64(currentUsage.Stime.Usec-p.usage.Stime.Usec) >> microsecondBitShift

	p.usage = &currentUsage

	props[userCPUSeconds] = strconv.FormatInt(userTime, 10)
	props[sysCPUSeconds] = strconv.FormatInt(sysTime, 10)

	return props, nil
}
