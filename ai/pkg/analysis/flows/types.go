package flows

import (
	"errors"
	"fmt"
	"strings"

	flowpb "github.com/cilium/cilium/api/v1/flow"
)

var ErrNoEndpointName = errors.New("no endpoint name")

type Connection struct {
	Pod1  string
	Pod2  string
	Key   string
	Flows []*flowpb.Flow
}

type FlowSummary map[string]*Connection

func (fs FlowSummary) FormatForLM() string {
	// FIXME hacky right now
	forwards := fs.connStrings(flowpb.Verdict_FORWARDED)
	drops := fs.connStrings(flowpb.Verdict_DROPPED)
	other := fs.connStrings(flowpb.Verdict_VERDICT_UNKNOWN)

	return fmt.Sprintf("SUCCESSFUL CONNECTIONS:\n%s\n\nDROPPED CONNECTIONS:\n%s\n\nOTHER CONNECTIONS:\n%s", forwards, drops, other)
}

func (fs FlowSummary) connStrings(verdict flowpb.Verdict) string {
	connStrings := make([]string, 0, len(fs))
	for _, conn := range fs {
		match := false
		for _, f := range conn.Flows {
			// FIXME hacky right now
			if f.GetVerdict() == verdict || (verdict == flowpb.Verdict_VERDICT_UNKNOWN && f.GetVerdict() != flowpb.Verdict_FORWARDED && f.GetVerdict() != flowpb.Verdict_DROPPED) {
				match = true
				break
			}
		}

		if !match {
			continue
		}

		connString := fmt.Sprintf("Connection: %s -> %s, Number of Flows: %d", conn.Pod1, conn.Pod2, len(conn.Flows))
		connStrings = append(connStrings, connString)
	}

	if len(connStrings) == 0 {
		return "none"
	}

	return strings.Join(connStrings, "\n")
}
