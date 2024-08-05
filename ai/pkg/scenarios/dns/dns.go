package dns

import (
	"context"

	"github.com/microsoft/retina/ai/pkg/lm"
	"github.com/microsoft/retina/ai/pkg/scenarios"
)

var (
	Definition = scenarios.NewDefinition("DNS", "DNS", parameterSpecs, &handler{})

	dnsQuery = &scenarios.ParameterSpec{
		Name:        "dnsQuery",
		DataType:    "string",
		Description: "DNS query",
		Optional:    true,
	}

	parameterSpecs = []*scenarios.ParameterSpec{
		scenarios.Namespace1,
		scenarios.Namespace2,
		dnsQuery,
	}
)

// mirrored with parameterSpecs
type params struct {
	Namespace1 string
	Namespace2 string
	DNSQuery   string
}

type handler struct{}

func (h *handler) Handle(ctx context.Context, cfg *scenarios.Config, typedParams map[string]any, question string, history lm.ChatHistory) (string, error) {
	// TODO
	return "", nil
}
