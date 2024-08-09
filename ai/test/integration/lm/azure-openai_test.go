package lmtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/microsoft/retina/ai/pkg/lm"
	"github.com/sirupsen/logrus"
)

func TestAzureOpenAICompletion(t *testing.T) {
	log := logrus.New()

	// configure LM (language model)
	// model := lm.NewEchoModel()
	model, err := lm.NewAzureOpenAI()
	if err != nil {
		log.WithError(err).Fatal("failed to create Azure OpenAI model")
	}
	log.Info("initialized Azure OpenAI model")

	resp, err := model.Generate(context.TODO(), `You are an assistant with expertise in Kubernetes Networking. The user is debugging networking issues on their Pods and/or Nodes. Provide a succinct summary identifying any issues in the "summary of network flow logs" provided by the user.`, nil, `summary of network flow logs:
	abc`)
	if err != nil {
		log.WithError(err).Fatal("error calling llm")
	}
	log.Info("called llm")
	fmt.Println(resp)
}
