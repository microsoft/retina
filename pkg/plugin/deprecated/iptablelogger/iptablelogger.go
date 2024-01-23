// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package iptablelogger

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"golang.org/x/sys/unix"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cc clang-14 -cflags "-g -O2 -Wall -D__TARGET_ARCH_x86 -Wall" -target bpf -type verdict kprobe ./cprog/iptable_logger.c -- -I../common/include -I../common/libbpf/src

func readEvent(ipl *iptableLogger, event chan<- kprobeVerdict) {
	var data kprobeVerdict
	for {
		record, err := ipl.verdictRd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			ipl.l.Error("reading from perf event reader: %w", zap.Error(err))
			continue
		}

		if record.LostSamples != 0 {
			ipl.l.Warn("perf event ring buffer full:", zap.Uint64("dropped samples", record.LostSamples))
			continue
		}

		// Parse the perf event entry into a bpfEvent structure.
		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &data); err != nil {
			ipl.l.Error("parsing perf event: %w", zap.Error(err))
			continue
		}

		event <- data
	}
}

func notifyVerdictData(ipl *iptableLogger, event kprobeVerdict) {
	saddrbuf := make([]byte, 4)
	daddrbuf := make([]byte, 4)

	binary.LittleEndian.PutUint32(saddrbuf, uint32(event.Flow.Saddr))
	binary.LittleEndian.PutUint32(daddrbuf, uint32(event.Flow.Daddr))

	srcip := net.IPv4(saddrbuf[0], saddrbuf[1], saddrbuf[2], saddrbuf[3])
	dstip := net.IPv4(daddrbuf[0], daddrbuf[1], daddrbuf[2], daddrbuf[3])

	var b []byte

	for _, v := range event.Comm {
		b = append(b, v)
	}

	tcpData := &IPTableEvent{
		Timestamp:   event.Ts,
		Pid:         event.Pid,
		ProcessName: unix.ByteSliceToString(b),
		SrcIP:       srcip,
		DstIP:       dstip,
		SrcPort:     event.Flow.Sport,
		DstPort:     event.Flow.Dport,
		Verdict:     event.Status,
		Hook:        Chain(event.Flow.Hook),
	}

	pluginEvent := api.PluginEvent{
		Name:  Name,
		Event: tcpData,
	}

	// notify event channel about packet
	ipl.event <- pluginEvent
}

// NewTcpTracerPlugin Initialize logger object
func New(logger *log.ZapLogger) api.Plugin {
	return &iptableLogger{
		l: logger,
	}
}

func NewPluginFn(l *log.ZapLogger) api.Plugin {
	return New(l)
}

func (ipl *iptableLogger) Init(pluginEvent chan<- api.PluginEvent) error {
	nfhookslowfn := "nf_hook_slow"

	if err := rlimit.RemoveMemlock(); err != nil {
		ipl.l.Error("RemoveMemLock failed:%w", zap.Error(err))
		return err
	}

	objs := kprobeObjects{}
	if err := loadKprobeObjects(&objs, nil); err != nil {
		ipl.l.Error("loading objects: %w", zap.Error(err))
		return err
	}

	defer objs.Close()

	var err error
	ipl.Knfhook, err = link.Kprobe(nfhookslowfn, objs.NfHookSlow, nil)
	if err != nil {
		ipl.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	ipl.Kretnfhook, err = link.Kretprobe(nfhookslowfn, objs.NfHookSlowRet, nil)
	if err != nil {
		ipl.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	ipl.verdictRd, err = perf.NewReader(objs.Verdicts, os.Getpagesize())
	if err != nil {
		ipl.l.Error("creating perf event reader: %w", zap.Error(err))
		return err
	}

	ipl.event = pluginEvent
	return err
}

func (ipl *iptableLogger) Start(ctx context.Context) error {
	ipl.l.Info("Listening for events..")
	go waitOnEvent(ipl)
	return nil
}

func waitOnEvent(ipl *iptableLogger) {
	verdictEvent := make(chan kprobeVerdict, 500)
	go readEvent(ipl, verdictEvent)

	for {
		select {
		case verdictData := <-verdictEvent:
			notifyVerdictData(ipl, verdictData)
		}
	}
}

func (ipl *iptableLogger) Stop() error {
	ipl.l.Info("Closing probes...")
	ipl.Knfhook.Close()
	ipl.Kretnfhook.Close()
	ipl.verdictRd.Close()
	return nil
}
