package chat

import (
	"fmt"
	"strings"

	"github.com/microsoft/retina/ai/pkg/lm"
	"github.com/microsoft/retina/ai/pkg/scenarios"
	"github.com/microsoft/retina/ai/pkg/scenarios/dns"
	"github.com/microsoft/retina/ai/pkg/scenarios/drops"
)

const selectionSystemPrompt = "Select a scenario"

var (
	definitions = []*scenarios.Definition{
		drops.Definition,
		dns.Definition,
	}
)

func selectionPrompt(question string, history lm.ChatHistory) string {
	// TODO include parameters etc. and reference the user chat as context
	var sb strings.Builder
	sb.WriteString("Select a scenario:\n")
	for i, d := range definitions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, d.Name))
	}
	return sb.String()
}
