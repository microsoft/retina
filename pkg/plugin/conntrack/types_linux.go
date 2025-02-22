package conntrack

import (
	"strings"
	"time"

	"github.com/cilium/ebpf"
	"github.com/microsoft/retina/pkg/log"
)

const (
	defaultGCFrequency    = 15 * time.Second
	bpfSourceDir          = "_cprog"
	bpfSourceFileName     = "conntrack.c"
	dynamicHeaderFileName = "dynamic.h"
)

// Conntrack represents the conntrack plugin
type Conntrack struct {
	l           *log.ZapLogger
	objs        *conntrackObjects
	ctMap       *ebpf.Map
	gcFrequency time.Duration
}

// Define TCP flag constants
const (
	TCP_FIN = 0x01 // nolint:revive // Acceptable as flag
	TCP_SYN = 0x02 // nolint:revive // Acceptable as flag
	TCP_RST = 0x04 // nolint:revive // Acceptable as flag
	TCP_PSH = 0x08 // nolint:revive // Acceptable as flag
	TCP_ACK = 0x10 // nolint:revive // Acceptable as flag
	TCP_URG = 0x20 // nolint:revive // Acceptable as flag
	TCP_ECE = 0x40 // nolint:revive // Acceptable as flag
	TCP_CWR = 0x80 // nolint:revive // Acceptable as flag
)

// decodeFlags decodes the TCP flags into a human-readable string
func decodeFlags(flags uint8) string {
	var flagDescriptions []string
	if flags&TCP_FIN != 0 {
		flagDescriptions = append(flagDescriptions, "FIN")
	}
	if flags&TCP_SYN != 0 {
		flagDescriptions = append(flagDescriptions, "SYN")
	}
	if flags&TCP_RST != 0 {
		flagDescriptions = append(flagDescriptions, "RST")
	}
	if flags&TCP_PSH != 0 {
		flagDescriptions = append(flagDescriptions, "PSH")
	}
	if flags&TCP_ACK != 0 {
		flagDescriptions = append(flagDescriptions, "ACK")
	}
	if flags&TCP_URG != 0 {
		flagDescriptions = append(flagDescriptions, "URG")
	}
	if flags&TCP_ECE != 0 {
		flagDescriptions = append(flagDescriptions, "ECE")
	}
	if flags&TCP_CWR != 0 {
		flagDescriptions = append(flagDescriptions, "CWR")
	}
	if len(flagDescriptions) == 0 {
		return "None"
	}
	return strings.Join(flagDescriptions, ", ")
}

func decodeProto(proto uint8) string {
	switch proto {
	case 6: // nolint:gomnd // TCP
		return "TCP"
	case 17: // nolint:gomnd // UDP
		return "UDP"
	default:
		return "Not supported"
	}
}
