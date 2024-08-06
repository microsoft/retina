package chat

import (
	"context"
	"fmt"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowretrieval "github.com/microsoft/retina/ai/pkg/retrieval/flows"
	"github.com/microsoft/retina/ai/pkg/scenarios"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Bot struct {
	log           logrus.FieldLogger
	config        *rest.Config
	clientset     *kubernetes.Clientset
	model         lm.Model
	flowRetriever *flowretrieval.Retriever
}

// input log, config, clientset, model
func NewBot(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset, model lm.Model, useFlowsFromFile bool) *Bot {
	b := &Bot{
		log:           log.WithField("component", "chat"),
		config:        config,
		clientset:     clientset,
		model:         model,
		flowRetriever: flowretrieval.NewRetriever(log, config, clientset),
	}

	if useFlowsFromFile {
		b.flowRetriever.UseFile()
	}

	return b
}

func (b *Bot) HandleScenario(question string, history lm.ChatHistory, definition *scenarios.Definition, parameters map[string]string) (lm.ChatHistory, error) {
	if definition == nil {
		return history, fmt.Errorf("no scenario selected")
	}

	cfg := &scenarios.Config{
		Log:           b.log,
		Config:        b.config,
		Clientset:     b.clientset,
		Model:         b.model,
		FlowRetriever: b.flowRetriever,
	}

	ctx := context.TODO()
	response, err := definition.Handle(ctx, cfg, parameters, question, history)
	if err != nil {
		return history, fmt.Errorf("error handling scenario: %w", err)
	}

	history = append(history, lm.MessagePair{
		User:      question,
		Assistant: response,
	})

	return history, nil
}

// FIXME get user input and implement scenario selection
func (b *Bot) Loop() error {
	var history lm.ChatHistory

	for {
		// TODO get user input
		question := "what's wrong with my app?"

		// select scenario and get parameters
		definition, params, err := b.selectScenario(question, history)
		if err != nil {
			return fmt.Errorf("error selecting scenario: %w", err)
		}

		newHistory, err := b.HandleScenario(question, history, definition, params)
		if err != nil {
			return fmt.Errorf("error handling scenario: %w", err)
		}

		fmt.Println(newHistory[len(newHistory)-1].Assistant)

		history = newHistory
	}
}

// FIXME fix prompts
func (b *Bot) selectScenario(question string, history lm.ChatHistory) (*scenarios.Definition, map[string]string, error) {
	ctx := context.TODO()
	response, err := b.model.Generate(ctx, selectionSystemPrompt, nil, selectionPrompt(question, history))
	if err != nil {
		return nil, nil, fmt.Errorf("error generating response: %w", err)
	}

	// TODO parse response and return scenario definition and parameters
	_ = response
	return nil, nil, nil
}
