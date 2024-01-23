// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcpsend

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

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cc clang-14 -cflags "-g -O2 -Wall -D__TARGET_ARCH_x86 -Wall" -target bpf -type tcpsendEvent kprobe ./cprog/tcpsend.c -- -I../common/include -I../common/libbpf/src

// pull from hashmap in 10 second intervals to collect send data
// clear map at the end of each map iteration
func pullMapData(s *tcpsend, hmap *ebpf.Map) {
	var data kprobeMapkey
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		var key []byte
		var value uint64

		iter := hmap.Iterate()
		for iter.Next(&key, &value) {
			if err := binary.Read(bytes.NewBuffer(key), binary.LittleEndian, &data); err != nil {
				s.l.Error("parse map values error", zap.Error(err))
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
				s.l.Debug("srcip or dstip == 127.0.0.1")
				continue
			}

			if srcip.String() == constants.ZeroIP || dstip.String() == constants.ZeroIP {
				s.l.Debug("skipping IP 0.0.0.0")
				continue
			}

			if srcip.String() == dstip.String() {
				s.l.Debug("src and dst IP are the same")
				continue
			}

			sendData := &TcpsendData{
				LocalIP:    srcip,
				RemoteIP:   dstip,
				LocalPort:  data.Sport,
				RemotePort: data.Dport,
				L4Proto:    protocol,
				Sent:       ByteCount,
				Op:         "SEND",
			}

			notifySendData(s, sendData)
		}
		// clearing map
		iterClear := hmap.Iterate()
		for iterClear.Next(&key, &value) {
			err := hmap.Delete(&key)
			if err != nil {
				s.l.Error("Deleting map key failed", zap.Error(err))
			}
		}
	}
}

func notifySendData(s *tcpsend, sendData *TcpsendData) {
	pluginEvent := api.PluginEvent{
		Name:  Name,
		Event: sendData,
	}
	// notify event channel about packet
	s.event <- pluginEvent
}

// NewtcpsendPlugin Initialize logger object
func New(logger *log.ZapLogger) api.Plugin {
	return &tcpsend{
		l: logger,
	}
}

func NewPluginFn(l *log.ZapLogger) api.Plugin {
	return New(l)
}

func (s *tcpsend) Init(pluginEvent chan<- api.PluginEvent) error {
	s.l.Info("Entering tcpsend Init...")
	sendfn := "tcp_sendmsg"

	if err := rlimit.RemoveMemlock(); err != nil {
		s.l.Error("RemoveMemLock failed:%w", zap.Error(err))
		return err
	}
	objs := kprobeObjects{}
	if err := loadKprobeObjects(&objs, nil); err != nil {
		s.l.Error("loading objects: %w", zap.Error(err))
		return err
	}

	var err error
	s.kSend, err = link.Kprobe(sendfn, objs.TcpSendmsg, nil)
	if err != nil {
		s.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	s.hashmapData = objs.Mapevent
	s.event = pluginEvent
	s.l.Info("Exiting tcpsend Init...")
	return err
}

func (s *tcpsend) Start(ctx context.Context) error {
	s.l.Info("Start listening for send events...")
	go pullMapData(s, s.hashmapData)
	s.l.Info("Exiting tcpsend start...")
	return nil
}

func (s *tcpsend) Stop() error {
	s.l.Info("Closing probes...")
	s.kSend.Close()
	s.hashmapData.Close()
	s.l.Info("Exiting tcpsend Stop...")
	return nil
}
