// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcptracer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cc clang-14 -cflags "-g -O2 -Wall -D__TARGET_ARCH_x86 -Wall" -target bpf -type tcpv4event kprobe ./cprog/tcptracer.c -- -I../common/include -I../common/libbpf/src

func readEvent(t *tcpTracer, event chan<- kprobeTcpv4event, eventRD *perf.Reader) {
	var data kprobeTcpv4event
	for {
		record, err := eventRD.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				return
			}
			t.l.Warn("reading from perf event reader: %w", zap.Error(err))
			continue
		}

		if record.LostSamples != 0 {
			t.l.Warn("perf event ring buffer full:", zap.Uint64("lostsamples", record.LostSamples))
			continue
		}

		// Parse the perf event entry into a bpfEvent structure.
		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &data); err != nil {
			t.l.Error("parsing perf event: %w", zap.Error(err))
			continue
		}

		event <- data
	}
}

func notifyTcpV4Data(t *tcpTracer, event kprobeTcpv4event) {
	saddrbuf := make([]byte, 4)
	daddrbuf := make([]byte, 4)

	binary.LittleEndian.PutUint32(saddrbuf, uint32(event.Saddr))
	binary.LittleEndian.PutUint32(daddrbuf, uint32(event.Daddr))

	srcip := net.IPv4(saddrbuf[0], saddrbuf[1], saddrbuf[2], saddrbuf[3])
	dstip := net.IPv4(daddrbuf[0], daddrbuf[1], daddrbuf[2], daddrbuf[3])

	if srcip.String() == "127.0.0.1" && dstip.String() == "127.0.0.1" {
		return
	}

	if srcip.String() == "0.0.0.0" || dstip.String() == "0.0.0.0" {
		t.l.Info("skipping IP 0.0.0.0")
		return
	}

	var b []byte
	for _, v := range event.Comm {
		b = append(b, v)
	}

	tcpData := &TcpV4Data{
		Timestamp:   event.Ts,
		Pid:         event.Pid,
		ProcessName: unix.ByteSliceToString(b),
		LocalIP:     srcip,
		RemoteIP:    dstip,
		LocalPort:   event.Sport,
		RemotePort:  event.Dport,
		Sent:        event.SentBytes,
		Recv:        event.RecvBytes,
		Op:          Operation(event.Operation),
	}

	pluginEvent := api.PluginEvent{
		Name:  Name,
		Event: tcpData,
	}

	// notify event channel about packet
	t.event <- pluginEvent
}

// NewTcpTracerPlugin Initialize logger object
func New(logger *log.ZapLogger) api.Plugin {
	return &tcpTracer{
		l: logger,
	}
}

func NewPluginFn(l *log.ZapLogger) api.Plugin {
	return New(l)
}

func (t *tcpTracer) Init(pluginEvent chan<- api.PluginEvent) error {
	connectfn := "tcp_v4_connect"
	acceptfn := "inet_csk_accept"
	closefn := "tcp_close"

	if err := rlimit.RemoveMemlock(); err != nil {
		t.l.Error("RemoveMemLock failed:%w", zap.Error(err))
		return err
	}

	objs := kprobeObjects{}
	if err := loadKprobeObjects(&objs, nil); err != nil {
		t.l.Error("loading objects: %w", zap.Error(err))
		return err
	}

	defer objs.Close()

	var err error
	t.kConnect, err = link.Kprobe(connectfn, objs.TcpV4Connect, nil)
	if err != nil {
		t.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	t.kretConnect, err = link.Kretprobe(connectfn, objs.TcpV4ConnectRet, nil)
	if err != nil {
		t.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	t.kretAccept, err = link.Kretprobe(acceptfn, objs.InetCskAcceptRet, nil)
	if err != nil {
		t.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	t.kClose, err = link.Kprobe(closefn, objs.TcpClose, nil)
	if err != nil {
		t.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	t.tcpv4Rdaccept, err = perf.NewReader(objs.Tcpv4accept, os.Getpagesize())
	if err != nil {
		t.l.Error("creating perf event reader: %w", zap.Error(err))
		return err
	}

	t.tcpv4Rdconnect, err = perf.NewReader(objs.Tcpv4connect, os.Getpagesize())
	if err != nil {
		t.l.Error("creating perf event reader: %w", zap.Error(err))
		return err
	}

	t.tcpv4Rdclose, err = perf.NewReader(objs.Tcpv4close, os.Getpagesize())
	if err != nil {
		t.l.Error("creating perf event reader: %w", zap.Error(err))
		return err
	}

	t.event = pluginEvent
	return err
}

func (t *tcpTracer) Start(ctx context.Context) error {
	t.l.Info("Listening for tcp connect events...")
	go waitOnEvent(t)
	return nil
}

func waitOnEvent(t *tcpTracer) {
	tcpv4AcceptEvent := make(chan kprobeTcpv4event, 500)
	tcpv4ConnectEvent := make(chan kprobeTcpv4event, 500)
	tcpv4CloseEvent := make(chan kprobeTcpv4event, 500)
	go readEvent(t, tcpv4AcceptEvent, t.tcpv4Rdaccept)
	go readEvent(t, tcpv4ConnectEvent, t.tcpv4Rdconnect)
	go readEvent(t, tcpv4CloseEvent, t.tcpv4Rdclose)

	for {
		select {
		case tcpv4AcceptData := <-tcpv4AcceptEvent:
			notifyTcpV4Data(t, tcpv4AcceptData)
		case tcpv4ConnectData := <-tcpv4ConnectEvent:
			notifyTcpV4Data(t, tcpv4ConnectData)
		case tcpv4CloseData := <-tcpv4CloseEvent:
			notifyTcpV4Data(t, tcpv4CloseData)
		}
	}
}

func (t *tcpTracer) Stop() error {
	t.l.Info("Closing probes...")
	t.kConnect.Close()
	t.kretConnect.Close()
	t.kretAccept.Close()
	t.kClose.Close()
	t.tcpv4Rdaccept.Close()
	t.tcpv4Rdconnect.Close()
	t.tcpv4Rdclose.Close()
	return nil
}
