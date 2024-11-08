// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package common contains common functions and types used by all Retina plugins.
package common

import (
	"errors"
	"os"
	"syscall"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/google/gopacket/layers"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

//go:generate mockgen -destination=mocks/mock_types.go -package=mocks . ITracer

// Interface for IG tracers.
// Ref: https://pkg.go.dev/github.com/inspektor-gadget/inspektor-gadget@v0.18.1/pkg/gadgets/trace/dns/tracer#Tracer
type ITracer interface {
	SetEventHandler(interface{})
	Attach(pid uint32) error
	Detach(pid uint32) error
	Close()
}

// Interface for IG event handlers. Maps to cilum Flow.
func ProtocolToFlow(protocol string) int {
	switch protocol {
	case "tcp":
		return unix.IPPROTO_TCP
	case "udp":
		return unix.IPPROTO_UDP
	default:
		return 0
	}
}

// Refer: https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-6
// Inspektor gadget uses Gopacket pkg for DNS response codes.
// Ref: https://github.com/google/gopacket/blob/32ee38206866f44a74a6033ec26aeeb474506804/layers/dns.go#L129
func RCodeToFlow(rCode string) uint32 {
	switch rCode {
	case layers.DNSResponseCodeNoErr.String():
		return 0
	case layers.DNSResponseCodeFormErr.String():
		return 1
	case layers.DNSResponseCodeServFail.String():
		return 2
	case layers.DNSResponseCodeNXDomain.String():
		return 3
	case layers.DNSResponseCodeNotImp.String():
		return 4
	case layers.DNSResponseCodeRefused.String():
		return 5
	}
	return 24
}

// NewPerfReader creates a new perf reader with a buffer size that is a power of 2.
// It starts with the maximum buffer size and tries with smaller buffer sizes if it fails with ENOMEM.
// Returns an error if it fails to create a perf reader or encounters any other error.
// Not enforced, but max and min should be a power of 2.
// The allocated size will be a multiple of pagesize determined at runtime.
func NewPerfReader(l *log.ZapLogger, m *ebpf.Map, max, min int) (*perf.Reader, error) {
	var enomem error = syscall.ENOMEM
	for i := max; i >= min; i = i / 2 {
		r, err := perf.NewReader(m, i*os.Getpagesize())
		if err == nil {
			l.Info("perf reader created", zap.Any("Map", m.String()), zap.Int("PageSize", os.Getpagesize()), zap.Int("BufferSize", i*os.Getpagesize()))
			return r, nil
		} else if errors.Is(err, enomem) {
			// If the error is ENOMEM, we can try with a smaller buffer size.
			// Give plugins a chance to run even if they cannot allocate the maximum buffer size at the start.
			// Can help with repeated OOM kills.
			// Linux mainly uses lazy allocation, That is to say what the underlying physical pages are allocated
			// when you touch them for the first time.
			// So, you may be able to allocate a buffer of X sizes, but when using it later, if the memory is under pressure,
			// the real allocation may fail and the OOM killer be triggered to free some memory, at the expense of killing a process.
			l.Warn("perf reader creation failed with ENOMEM", zap.Any("Map", m.String()), zap.Error(err), zap.Int("BufferSize", i*os.Getpagesize()))
			continue
		} else {
			return nil, err
		}
	}
	return nil, errors.New("failed to create perf reader")
}
