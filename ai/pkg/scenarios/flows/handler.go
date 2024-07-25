package flows

import (
	"context"
	"fmt"
	"time"

	flowanalysis "github.com/microsoft/retina/ai/pkg/analysis/flows"
	"github.com/microsoft/retina/ai/pkg/lm"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/util"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type FlowScenario string

const (
	AnyScenario  FlowScenario = "Any"
	DropScenario FlowScenario = "Drops"
	DnsScenario  FlowScenario = "DNS"
)

type ScenarioParams struct {
	Scenario FlowScenario

	// parameters (all optional?)
	DnsQuery   string
	Nodes      []string
	Namespace1 string
	PodPrefix1 string
	Namespace2 string
	PodPrefix2 string
}

type Handler struct {
	log logrus.FieldLogger
	r   *flowretrieval.Retriever
	p   *flowanalysis.Parser
	a   *flowanalysis.Analyzer
}

func NewHandler(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset, model lm.Model) *Handler {
	return &Handler{
		log: log.WithField("component", "flow-handler"),
		r:   flowretrieval.NewRetriever(log, config, clientset),
		p:   flowanalysis.NewParser(log),
		a:   flowanalysis.NewAnalyzer(log, model),
	}
}

func (h *Handler) Handle(ctx context.Context, question string, chat lm.ChatHistory, params *ScenarioParams) (string, error) {
	h.log.Info("handling flows scenario...")

	// get flows
	// h.r.UseFile()

	if err := h.r.Init(); err != nil {
		return "", fmt.Errorf("error initializing flow retriever: %w", err)
	}

	req := flowsRequest(params)
	flows, err := h.r.Observe(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error observing flows: %w", err)
	}
	h.log.Info("observed flows")

	// analyze flows
	h.p.Parse(flows)
	summary := h.p.Summary()

	analyzeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := h.a.Analyze(analyzeCtx, question, chat, summary)
	if err != nil {
		return "", fmt.Errorf("error analyzing flows: %w", err)
	}
	h.log.Info("analyzed flows")

	// temporary printing
	fmt.Println("flow summary:")
	fmt.Println(summary.FormatForLM())
	fmt.Println()
	fmt.Println("response:")
	fmt.Println(resp)

	return resp, nil
}

// TODO DNS should not have a destination Pod (except maybe a specific coredns pod)
func flowsRequest(params *ScenarioParams) *observerpb.GetFlowsRequest {
	req := &observerpb.GetFlowsRequest{
		Number: util.MaxFlowsFromHubbleRelay,
		Follow: true,
	}

	if len(params.Nodes) == 0 {
		params.Nodes = nil
	}

	protocol := []string{"TCP", "UDP"}
	if params.Scenario == DnsScenario {
		protocol = []string{"DNS"}
	}

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

	req.Whitelist = nil

	req.Blacklist = []*flowpb.FlowFilter{
		{
			SourcePod: []string{"kube-system/"},
		},
		{
			DestinationPod: []string{"kube-system/"},
		},
	}

	return req
}
