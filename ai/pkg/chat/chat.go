package chat

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/scenarios"
	"github.com/microsoft/retina/ai/pkg/scenarios/dns"
	"github.com/microsoft/retina/ai/pkg/scenarios/drops"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	definitions = []*scenarios.Definition{
		drops.Definition,
		dns.Definition,
	}
)

type Bot struct {
	log       logrus.FieldLogger
	config    *rest.Config
	clientset *kubernetes.Clientset
	model     lm.Model
}

// input log, config, clientset, model
func NewBot(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset, model lm.Model) *Bot {
	return &Bot{
		log:       log.WithField("component", "chat"),
		config:    config,
		clientset: clientset,
		model:     model,
	}
}

func (b *Bot) Loop() error {
	var history lm.ChatHistory
	flowRetriever := flowretrieval.NewRetriever(b.log, b.config, b.clientset)

	for {
		// TODO get user input
		question := "what's wrong with my app?"

		// select scenario and get parameters
		definition, params, err := b.selectScenario(question, history)
		if err != nil {
			return fmt.Errorf("error selecting scenario: %w", err)
		}

		// cfg.FlowRetriever.UseFile()

		cfg := &scenarios.Config{
			Log:           b.log,
			Config:        b.config,
			Clientset:     b.clientset,
			Model:         b.model,
			FlowRetriever: flowRetriever,
		}

		ctx := context.TODO()
		response, err := definition.Handle(ctx, cfg, params, question, history)
		if err != nil {
			return fmt.Errorf("error handling scenario: %w", err)
		}

		fmt.Println(response)

		// TODO keep chat loop going
		break
	}

	return nil
}

func (b *Bot) selectScenario(question string, history lm.ChatHistory) (*scenarios.Definition, map[string]string, error) {
	// TODO use chat interface
	// FIXME hard-coding the scenario and params for now
	d := definitions[0]
	params := map[string]string{
		scenarios.Namespace1.Name: "default",
		scenarios.Namespace2.Name: "default",
	}

	return d, params, nil
}
