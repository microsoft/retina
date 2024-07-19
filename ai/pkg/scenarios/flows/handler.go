package flows

import (
	"context"
	"fmt"
	"time"

	flowanalysis "github.com/microsoft/retina/ai/pkg/analysis/flows"
	"github.com/microsoft/retina/ai/pkg/lm"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/util"

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
	if err := h.r.Init(); err != nil {
		return "", fmt.Errorf("error initializing flow retriever: %w", err)
	}

	flows, err := h.r.Observe(ctx, util.MaxFlowsToAnalyze)
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
