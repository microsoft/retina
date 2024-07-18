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
	connStrings := make([]string, 0, len(fs))
	for _, conn := range fs {
		connString := fmt.Sprintf("Connection: %s -> %s, Number of Flows: %d", conn.Pod1, conn.Pod2, len(conn.Flows))
		connStrings = append(connStrings, connString)
	}

	return strings.Join(connStrings, "\n")
}
