package flows

import (
	"errors"

	flowpb "github.com/cilium/cilium/api/v1/flow"
)

var (
	ErrNoEndpointName = errors.New("no endpoint name")
	ErrNilEndpoint    = errors.New("nil endpoint")
)

type Connection struct {
	Pod1 string
	Pod2 string
	Key  string

	// UDP   *UdpSummary
	// TCP   *TcpSummary
	Flows []*flowpb.Flow
}

type Connections map[string]*Connection

// func

// type UdpSummary struct {
// 	MinLatency   time.Duration
// 	MaxLatency   time.Duration
// 	AvgLatency   time.Duration
// 	TotalPackets int
// 	TotalBytes   int
// }

// type TcpSummary struct {
// 	MinLatency   time.Duration
// 	MaxLatency   time.Duration
// 	AvgLatency   time.Duration
// 	TotalPackets int
// 	TotalBytes   int
// 	*TcpFlagSummary
// }

// type TcpFlagSummary struct {
// 	SynCount    int
// 	AckCount    int
// 	SynAckCount int
// 	FinCount    int
// 	RstCount    int
// }

// type FlowSummary map[string]*Connection

// func (fs FlowSummary) Aggregate() {
// 	for _, conn := range fs {
// 		udpTimestamps := make(map[string][]*timestamppb.Timestamp)
// 		tcpTimestamps := make(map[string][]*timestamppb.Timestamp)
// 		for _, f := range conn.Flows {
// 			l4 := f.GetL4()
// 			if l4 == nil {
// 				continue
// 			}

// 			udp := l4.GetUDP()
// 			if udp != nil {
// 				if conn.UDP == nil {
// 					conn.UDP = &UdpSummary{}
// 				}

// 				conn.UDP.TotalPackets += 1

// 				src, err := endpointName(f.GetSource())
// 				if err != nil {
// 					// FIXME warn and continue
// 					log.Fatalf("bad src endpoint while aggregating: %w", err)
// 				}
// 				dst, err := endpointName(f.GetDestination())
// 				if err != nil {
// 					// FIXME warn and continue
// 					log.Fatalf("bad dst endpoint while aggregating: %w", err)
// 				}

// 				tuple := fmt.Sprintf("%s:%d -> %s:%d", src, udp.GetSourcePort(), dst, udp.GetDestinationPort())

// 				time := f.GetTime()
// 				if time == nil {
// 					// FIXME warn and continue
// 					log.Fatalf("nil time while aggregating")
// 				}

// 				udpTimestamps[tuple] = append(udpTimestamps[tuple], f.GetTime())
// 			}

// 			tcp := l4.GetTCP()
// 			if tcp != nil {
// 				if conn.TCP == nil {
// 					conn.TCP = &TcpSummary{}
// 				}

// 				conn.TCP.TotalPackets += 1

// 				if conn.TCP.TcpFlagSummary == nil {
// 					conn.TCP.TcpFlagSummary = &TcpFlagSummary{}
// 				}

// 				flags := tcp.GetFlags()
// 				if flags == nil {
// 					// FIXME warn and continue
// 					log.Fatalf("nil flags while aggregating")
// 				}

// 				switch {
// 					case flags.SYN && flags.ACK:
// 						conn.TCP.TcpFlagSummary.SynAckCount += 1
// 					case flags.SYN:
// 						conn.TCP.TcpFlagSummary.SynCount += 1
// 					case flags.ACK:
// 						conn.TCP.TcpFlagSummary.AckCount += 1
// 					case flags.FIN:
// 						conn.TCP.TcpFlagSummary.FinCount += 1
// 					case flags.RST:
// 						conn.TCP.TcpFlagSummary.RstCount += 1
// 				}

// 				src, err := endpointName(f.GetSource())
// 				if err != nil {
// 					// FIXME warn and continue
// 					log.Fatalf("bad src endpoint while aggregating: %w", err)
// 				}
// 				dst, err := endpointName(f.GetDestination())
// 				if err != nil {
// 					// FIXME warn and continue
// 					log.Fatalf("bad dst endpoint while aggregating: %w", err)
// 				}

// 				tuple := fmt.Sprintf("%s:%d -> %s:%d", src, udp.GetSourcePort(), dst, udp.GetDestinationPort())

// 				time := f.GetTime()
// 				if time == nil {
// 					// FIXME warn and continue
// 					log.Fatalf("nil time while aggregating")
// 				}

// 				tcpTimestamps[tuple] = append(tcpTimestamps[tuple], f.GetTime())
// 			}
// 		}
// 	}
// }
