// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build ebpf && linux

// Tests for the tcpretrans BPF tracepoint program.
//
// These load the compiled BPF program, verify it passes the kernel verifier,
// and confirm it can attach to the tcp/tcp_retransmit_skb tracepoint.
//
// Unlike socket-filter or TC programs, tracepoint programs cannot be exercised
// with synthetic packets via BPF_PROG_TEST_RUN because the handler dereferences
// kernel pointers (skaddr, skbaddr) that cannot be mocked from userspace.
// The load+attach tests still provide significant value: they catch verifier
// regressions, CO-RE relocation failures, and missing tracepoints.
//
// Requires: root (or CAP_BPF+CAP_SYS_ADMIN), Linux kernel 5.8+
// (bpf_ktime_get_boot_ns helper, commit 71d19214776e).
// Run: sudo go test -tags=ebpf -v -count=1 ./pkg/plugin/tcpretrans/...

package tcpretrans

import (
	"os"
	"testing"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/ebpftest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestObjects(t *testing.T) *tcpretransObjects {
	t.Helper()
	ebpftest.RequirePrivileged(t)

	spec, err := loadTcpretrans()
	require.NoError(t, err)
	ebpftest.RemoveMapPinning(spec)

	var objs tcpretransObjects
	err = spec.LoadAndAssign(&objs, nil)
	require.NoError(t, err)
	t.Cleanup(func() { objs.Close() })
	return &objs
}

// TestBPFLoadAndVerify verifies the compiled BPF program passes the kernel
// verifier and all expected objects (program + perf event array) are created.
func TestBPFLoadAndVerify(t *testing.T) {
	objs := loadTestObjects(t)

	assert.NotNil(t, objs.RetinaTcpRetransmitSkb, "tracepoint program should be loaded")
	assert.NotNil(t, objs.RetinaTcpretransEvents, "perf event array map should be created")
}

// TestBPFTracepointAttach verifies the program can attach to the
// tcp/tcp_retransmit_skb tracepoint (stable since kernel 4.15, commit e086101b150a).
func TestBPFTracepointAttach(t *testing.T) {
	objs := loadTestObjects(t)

	tp, err := link.Tracepoint("tcp", "tcp_retransmit_skb", objs.RetinaTcpRetransmitSkb, nil)
	require.NoError(t, err, "should attach to tcp/tcp_retransmit_skb tracepoint")
	t.Cleanup(func() { tp.Close() })
}

// TestBPFPerfReaderCreate verifies a perf reader can be opened on the events map.
func TestBPFPerfReaderCreate(t *testing.T) {
	objs := loadTestObjects(t)

	reader, err := perf.NewReader(objs.RetinaTcpretransEvents, os.Getpagesize()*4)
	require.NoError(t, err, "should create perf reader")
	t.Cleanup(func() { reader.Close() })
}

// TestBPFStopAfterInitWithoutStart exercises the fixed resource-leak path:
// Init() loads BPF objects and attaches the tracepoint, but Start() is never
// called. Stop() must still release all kernel resources.
func TestBPFStopAfterInitWithoutStart(t *testing.T) {
	ebpftest.RequirePrivileged(t)

	log.SetupZapLogger(log.GetDefaultLogOpts())

	p := New(&kcfg.Config{EnablePodLevel: true})
	require.NoError(t, p.Init())
	// Start() deliberately not called.
	require.NoError(t, p.Stop(), "Stop() must clean up even without Start()")
}
