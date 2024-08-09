package dns

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowparsing "github.com/microsoft/retina/ai/pkg/parse/flows"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/scenarios"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
)

var (
	Definition = scenarios.NewDefinition("DNS", "DNS", parameterSpecs, &handler{})

	DNSQuery = &scenarios.ParameterSpec{
		Name:        "dnsQuery",
		DataType:    "string",
		Description: "DNS query",
		Optional:    true,
	}

	parameterSpecs = []*scenarios.ParameterSpec{
		scenarios.Namespace1,
		scenarios.PodPrefix1,
		scenarios.Namespace2,
		scenarios.PodPrefix2,
		scenarios.Nodes,
		DNSQuery,
	}
)

// mirrored with parameterSpecs
type params struct {
	Namespace1 string
	PodPrefix1 string
	Namespace2 string
	PodPrefix2 string
	Nodes      []string
	DNSQuery   string
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

// TODO handle dnsQuery param
func flowsRequest(params *params) *observerpb.GetFlowsRequest {
	req := &observerpb.GetFlowsRequest{
		Number: flowretrieval.MaxFlowsFromHubbleRelay,
		Follow: true,
	}

	protocol := []string{"DNS"}

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

	req.Whitelist = []*flowpb.FlowFilter{
		filterDirection1,
		filterDirection2,
	}

	return req
}

func formatFlowLogs(connections flowparsing.Connections) string {
	requestsWithoutResponse := make([]string, 0)
	successfulResponses := make([]string, 0)
	failedResponses := make([]string, 0)
	for _, conn := range connections {
		requests := make(map[string]struct{})
		responses := make(map[string]uint32)
		for _, f := range conn.Flows {
			if f.GetL7().GetDns() == nil {
				continue
			}

			dnsType := f.GetL7().Type.String()

			query := f.GetL7().GetDns().GetQuery()
			switch dnsType {
			case "REQUEST":
				requests[query] = struct{}{}
			case "RESPONSE":
				responses[query] = f.GetL7().GetDns().GetRcode()
			}
		}

		for q := range requests {
			if _, ok := responses[q]; !ok {
				line := fmt.Sprintf("Pods: %s. query: %s", conn.Key, q)
				requestsWithoutResponse = append(requestsWithoutResponse, line)
			}
		}

		for q, rcode := range responses {
			if rcode == 0 {
				line := fmt.Sprintf("Pods: %s. query: %s", conn.Key, q)
				successfulResponses = append(successfulResponses, line)
			} else {
				line := fmt.Sprintf("Pods: %s. query: %s. error: %s", conn.Key, q, rcodeToErrorName(rcode))
				failedResponses = append(failedResponses, line)
			}
		}
	}

	return fmt.Sprintf("SUCCESSFUL RESPONSES:\n%v\n\nRESPONSES WITH ERRORS:\n%v\n\nREQUESTS WITHOUT RESPONSES:\n%v", successfulResponses, failedResponses, requestsWithoutResponse)
}

func rcodeToErrorName(rcode uint32) string {
	switch rcode {
	case 0:
		return "NoError"
	case 1:
		return "FormErr"
	case 2:
		return "ServFail"
	case 3:
		return "NXDomain"
	case 4:
		return "NotImp"
	case 5:
		return "Refused"
	case 6:
		return "YXDomain"
	case 7:
		return "YXRRSet"
	case 8:
		return "NXRRSet"
	case 9:
		return "NotAuth"
	case 10:
		return "NotZone"
	case 11:
		return "DSOTYPENI"
	case 16:
		return "BADVERS/BADSIG"
	case 17:
		return "BADKEY"
	case 18:
		return "BADTIME"
	case 19:
		return "BADMODE"
	case 20:
		return "BADNAME"
	case 21:
		return "BADALG"
	case 22:
		return "BADTRUNC"
	case 23:
		return "BADCOOKIE"
	default:
		return "Unknown"
	}
}
