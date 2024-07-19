package main

import (
	"context"

	"github.com/microsoft/retina/ai/pkg/lm"
	flowscenario "github.com/microsoft/retina/ai/pkg/scenarios/flows"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeconfigPath = "/home/hunter/.kube/config"

// const kubeconfigPath = "C:\\Users\\hgregory\\.kube\\config"

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	log.Info("starting app...")

	// retrieve configs
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.WithError(err).Fatal("failed to get kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Fatal("failed to create clientset")
	}
	log.Info("retrieved kubeconfig and clientset")

	// configure LM (language model)
	// model := lm.NewEchoModel()
	model, err := lm.NewAzureOpenAI()
	if err != nil {
		log.WithError(err).Fatal("failed to create Azure OpenAI model")
	}
	log.Info("initialized Azure OpenAI model")

	handleChat(log, config, clientset, model)
}

// pretend there's input from chat interface
func handleChat(log logrus.FieldLogger, config *rest.Config, clientset *kubernetes.Clientset, model lm.Model) {
	question := "what's wrong with my app?"
	var chat lm.ChatHistory

	h := flowscenario.NewHandler(log, config, clientset, model)
	params := &flowscenario.ScenarioParams{
		Scenario:   flowscenario.AnyScenario,
		Namespace1: "frontend",
		Namespace2: "backend",
	}

	ctx := context.TODO()
	response, err := h.Handle(ctx, question, chat, params)
	if err != nil {
		log.WithError(err).Fatal("error running flows scenario")
	}

	_ = response
}
