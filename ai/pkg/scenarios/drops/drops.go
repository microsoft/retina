package drops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowparsing "github.com/microsoft/retina/ai/pkg/parse/flows"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/scenarios"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
)

var (
	Definition = scenarios.NewDefinition("DROPS", "DROPS", parameterSpecs, &handler{})

	parameterSpecs = []*scenarios.ParameterSpec{
		scenarios.Namespace1,
		scenarios.PodPrefix1,
		scenarios.Namespace2,
		scenarios.PodPrefix2,
		scenarios.Nodes,
	}
)

// mirrored with parameterSpecs
type params struct {
	Namespace1 string
	PodPrefix1 string
	Namespace2 string
	PodPrefix2 string
	Nodes      []string
}

type handler struct{}

func (h *handler) Handle(ctx context.Context, cfg *scenarios.Config, typedParams map[string]any, question string, history lm.ChatHistory) (string, error) {
	l := cfg.Log.WithField("scenario", "drops")
	l.Info("handling drops scenario...")

	if err := cfg.FlowRetriever.Init(); err != nil {
		return "", fmt.Errorf("error initializing flow retriever: %w", err)
	}

	params := &params{
		Namespace1: anyToString(typedParams[scenarios.Namespace1.Name]),
		PodPrefix1: anyToString(typedParams[scenarios.PodPrefix1.Name]),
		Namespace2: anyToString(typedParams[scenarios.Namespace2.Name]),
		PodPrefix2: anyToString(typedParams[scenarios.PodPrefix2.Name]),
		Nodes:      anyToStringSlice(typedParams[scenarios.Nodes.Name]),
	}

	req := flowsRequest(params)
	flows, err := cfg.FlowRetriever.Observe(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error observing flows: %w", err)
	}
	l.Info("observed flows")

	// analyze flows
	p := flowparsing.NewParser(l)
	p.Parse(flows)
	connections := p.Connections()

	formattedFlowLogs := formatFlowLogs(connections)

	message := fmt.Sprintf(messagePromptTemplate, question, formattedFlowLogs)
	analyzeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := cfg.Model.Generate(analyzeCtx, systemPrompt, history, message)
	if err != nil {
		return "", fmt.Errorf("error analyzing flows: %w", err)
	}
	l.Info("analyzed flows")

	return resp, nil
}

// cast to string without nil panics
func anyToString(a any) string {
	if a == nil {
		return ""
	}
	return a.(string)
}

// cast to []string without nil panics
func anyToStringSlice(a any) []string {
	if a == nil {
		return nil
	}
	return a.([]string)
}

func flowsRequest(params *params) *observerpb.GetFlowsRequest {
	req := &observerpb.GetFlowsRequest{
		Number: flowretrieval.MaxFlowsFromHubbleRelay,
		Follow: true,
	}

	protocol := []string{"TCP", "UDP"}

	if params.Namespace1 == "" && params.PodPrefix1 == "" && params.Namespace2 == "" && params.PodPrefix2 == "" {
		req.Whitelist = []*flowpb.FlowFilter{
			{
				NodeName: params.Nodes,
				Protocol: protocol,
			},
		}

		return req
	}

	var prefix1 []string
	if params.Namespace1 != "" || params.PodPrefix1 != "" {
		prefix1 = append(prefix1, fmt.Sprintf("%s/%s", params.Namespace1, params.PodPrefix1))
	}

	var prefix2 []string
	if params.Namespace2 != "" || params.PodPrefix2 != "" {
		prefix2 = append(prefix2, fmt.Sprintf("%s/%s", params.Namespace2, params.PodPrefix2))
	}

	filterDirection1 := &flowpb.FlowFilter{
		NodeName:       params.Nodes,
		SourcePod:      prefix1,
		DestinationPod: prefix2,
		Protocol:       protocol,
	}

	filterDirection2 := &flowpb.FlowFilter{
		NodeName:       params.Nodes,
		SourcePod:      prefix2,
		DestinationPod: prefix1,
		Protocol:       protocol,
	}

	// filterPod1ToIP := &flowpb.FlowFilter{
	// 	NodeName:      params.Nodes,
	// 	SourcePod:     prefix1,
	// 	DestinationIp: []string{"10.224.1.214"},
	// 	Protocol:      protocol,
	// }

	// filterPod1FromIP := &flowpb.FlowFilter{
	// 	NodeName:       params.Nodes,
	// 	SourceIp:       []string{"10.224.1.214"},
	// 	DestinationPod: prefix1,
	// 	Protocol:       protocol,
	// }

	// includes services
	// world := []string{"reserved:world"}

	// filterPod1ToWorld := &flowpb.FlowFilter{
	// 	NodeName:         params.Nodes,
	// 	SourcePod:        prefix1,
	// 	DestinationLabel: world,
	// 	Protocol:         protocol,
	// }

	// filterPod1FromWorld := &flowpb.FlowFilter{
	// 	NodeName:       params.Nodes,
	// 	SourceLabel:    world,
	// 	DestinationPod: prefix1,
	// 	Protocol:       protocol,
	// }

	req.Whitelist = []*flowpb.FlowFilter{
		filterDirection1,
		filterDirection2,
		// filterPod1FromIP,
		// filterPod1ToIP,
	}

	return req
}

func formatFlowLogs(connections flowparsing.Connections) string {
	// FIXME hacky right now
	forwards := connStrings(connections, flowpb.Verdict_FORWARDED)

	drops := connStrings(connections, flowpb.Verdict_DROPPED)
	other := connStrings(connections, flowpb.Verdict_VERDICT_UNKNOWN)

	return fmt.Sprintf("SUCCESSFUL CONNECTIONS:\n%s\n\nDROPPED CONNECTIONS:\n%s\n\nOTHER CONNECTIONS:\n%s", forwards, drops, other)
}

func connStrings(connections flowparsing.Connections, verdict flowpb.Verdict) string {
	connStrings := make([]string, 0, len(connections))
	for _, conn := range connections {
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

		connString := ""
		if verdict == flowpb.Verdict_FORWARDED && conn.Flows[0].L4.GetTCP() != nil {
			successful := false
			rst := false
			for _, f := range conn.Flows {
				if f.GetVerdict() == flowpb.Verdict_FORWARDED && f.L4.GetTCP().GetFlags().GetSYN() && f.L4.GetTCP().GetFlags().GetACK() {
					successful = true
					continue
				}

				if f.GetVerdict() == flowpb.Verdict_FORWARDED && f.L4.GetTCP().GetFlags().GetRST() {
					rst = true
					continue
				}
			}
			_ = successful
			connString = fmt.Sprintf("Connection: %s -> %s, Number of Flows: %d. Was Reset: %v", conn.Pod1, conn.Pod2, len(conn.Flows), rst)

		} else {

			connString = fmt.Sprintf("Connection: %s -> %s, Number of Flows: %d", conn.Pod1, conn.Pod2, len(conn.Flows))
		}

		connStrings = append(connStrings, connString)
	}

	if len(connStrings) == 0 {
		return "none"
	}

	return strings.Join(connStrings, "\n")
}
