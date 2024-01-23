// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build exclude

package tcpreceive

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/constants"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cc clang-14 -cflags "-g -O2 -Wall -D__TARGET_ARCH_x86 -Wall" -target bpf -type tcpreceiveEvent kprobe ./cprog/tcpreceive.c -- -I../common/include -I../common/libbpf/src

// pull from hashmap in 10 second intervals to collect send data
// clear map at the end of each map iteration
func pullMapData(r *tcpreceive, event chan<- kprobeTcpreceiveEvent, hmap *ebpf.Map) {
	var data kprobeMapkey
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		var key []byte
		var value int32

		iter := hmap.Iterate()
		for iter.Next(&key, &value) {
			if err := binary.Read(bytes.NewBuffer(key), binary.LittleEndian, &data); err != nil {
				r.l.Error("parse map values error", zap.Error(err))
			}

			ByteCount := value
			saddrbuf := make([]byte, 4)
			daddrbuf := make([]byte, 4)

			binary.LittleEndian.PutUint32(saddrbuf, uint32(data.Saddr))
			binary.LittleEndian.PutUint32(daddrbuf, uint32(data.Daddr))
			srcip := net.IPv4(saddrbuf[0], saddrbuf[1], saddrbuf[2], saddrbuf[3])
			dstip := net.IPv4(daddrbuf[0], daddrbuf[1], daddrbuf[2], daddrbuf[3])
			l4proto := data.L4proto
			var protocol string
			if l4proto == 6 {
				protocol = constants.TCP
			}

			if l4proto == 17 {
				protocol = constants.UDP
			}

			if srcip.String() == constants.LocalhostIP || dstip.String() == constants.LocalhostIP {
				r.l.Debug("srcip or dstip == 127.0.0.1")
				continue
			}

			if srcip.String() == constants.ZeroIP || dstip.String() == constants.ZeroIP {
				r.l.Debug("skipping IP 0.0.0.0")
				continue
			}

			if srcip.String() == dstip.String() {
				r.l.Debug("src and dst IP are the same")
				continue
			}

			recvData := &TcpreceiveData{
				LocalIP:    srcip,
				RemoteIP:   dstip,
				LocalPort:  data.Sport,
				RemotePort: data.Dport,
				L4Proto:    protocol,
				Recv:       ByteCount,
				Op:         "RECEIVE",
			}

			notifyRecvData(r, recvData)
		}
		// clearing map
		iterClear := hmap.Iterate()
		for iterClear.Next(&key, &value) {
			err := hmap.Delete(&key)
			if err != nil {
				r.l.Error("Deleting map key failed", zap.Error(err))
			}
		}
	}
}

func notifyRecvData(s *tcpreceive, recvData *TcpreceiveData) {
	pluginEvent := api.PluginEvent{
		Name:  Name,
		Event: recvData,
	}
	// notify event channel about packet
	s.event <- pluginEvent
}

// NewtcpreceivePlugin Initialize logger object
func New(logger *log.ZapLogger) api.Plugin {
	return &tcpreceive{
		l: logger,
	}
}

func NewPluginFn(l *log.ZapLogger) api.Plugin {
	return New(l)
}

func (r *tcpreceive) Init(pluginEvent chan<- api.PluginEvent) error {
	r.l.Info("Entering tcpreceive Init...")
	recvfn := "tcp_cleanup_rbuf"

	if err := rlimit.RemoveMemlock(); err != nil {
		r.l.Error("RemoveMemLock failed:%w", zap.Error(err))
		return err
	}

	objs := kprobeObjects{}
	if err := loadKprobeObjects(&objs, nil); err != nil {
		r.l.Error("loading objects: %w", zap.Error(err))
		return err
	}

	var err error
	r.kRecv, err = link.Kprobe(recvfn, objs.TcpCleanupRbuf, nil)
	if err != nil {
		r.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	r.hashmapData = objs.Mapevent
	r.event = pluginEvent
	r.l.Info("Exiting tcpreceive Init...")
	return err
}

func (r *tcpreceive) Start(ctx context.Context) error {
	r.l.Info("Start listening for receive events...")
	tcpreceiveEvent := make(chan kprobeTcpreceiveEvent, 500)
	go pullMapData(r, tcpreceiveEvent, r.hashmapData)
	r.l.Info("Exiting tcpreceive start...")
	return nil
}

func (r *tcpreceive) Stop() error {
	r.l.Info("Closing probes...")
	r.kRecv.Close()
	r.hashmapData.Close()
	r.l.Info("Exiting tcpreceive Stop...")
	return nil
}
