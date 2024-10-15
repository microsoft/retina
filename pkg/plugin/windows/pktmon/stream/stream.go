package stream

import (
	"context"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/google/gopacket"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
)

type WinPktMon struct {
	l *log.ZapLogger
}

func NewWinPktMonStreamer(l *log.ZapLogger, truncationSize, bufferSize, bufferMultiplier int) *WinPktMon {
	return &WinPktMon{
		l: l,
	}
}

func (w *WinPktMon) Initialize() error {
	return nil
}

func (w *WinPktMon) GetNextPacket(ctx context.Context) (*flow.Flow, *utils.RetinaMetadata, gopacket.Packet, error) {
	w.l.Info("pktmon plugin not implemented")
	<-ctx.Done()
	return nil, nil, nil, ctx.Err()
}

func (w *WinPktMon) ParseDNS(fl *flow.Flow, metadata *utils.RetinaMetadata, packet gopacket.Packet) error {
	return nil
}

func (w *WinPktMon) IncMissedWrite(missed int) {
}

func (w *WinPktMon) IncMissedRead(missed int) {
}

func (w *WinPktMon) PrintAndResetMissedWrite(sessionID string) {
}

func (w *WinPktMon) PrintAndResetMissedRead(sessionID string) {
}

func AddTcpFlagsBool(f *flow.Flow, syn, ack, fin, rst, psh, urg bool) {
}
