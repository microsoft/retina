package flows

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/ai/pkg/lm"
	"github.com/sirupsen/logrus"
)

type Analyzer struct {
	log   logrus.FieldLogger
	model lm.Model
}

func NewAnalyzer(log logrus.FieldLogger, model lm.Model) *Analyzer {
	return &Analyzer{
		log:   logrus.WithField("component", "flow-analyzer"),
		model: model,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, query string, chat lm.ChatHistory, summary FlowSummary) (string, error) {
	message := fmt.Sprintf(messagePromptTemplate, query, summary.FormatForLM())
	return a.model.Generate(ctx, systemPrompt, chat, message)
}
