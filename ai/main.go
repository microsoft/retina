package main

import (
	"fmt"
	"os/user"

	"github.com/microsoft/retina/ai/pkg/chat"
	"github.com/microsoft/retina/ai/pkg/lm"
	"github.com/microsoft/retina/ai/pkg/scenarios"
	"github.com/microsoft/retina/ai/pkg/scenarios/drops"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TODO incorporate this code into a CLI tool someday

type config struct {
	// currently supports "echo" or "AOAI"
	model string

	// optional. defaults to ~/.kube/config
	kubeconfigPath string

	// retrieved flows are currently written to ./flows.json
	useFlowsFromFile bool

	// eventually, the below should be optional once user input is implemented
	question string
	history  lm.ChatHistory

	// eventually, the below should be optional once scenario selection is implemented
	scenario   *scenarios.Definition
	parameters map[string]string
}

var defaultConfig = &config{
	model:            "echo", // echo or AOAI
	useFlowsFromFile: false,
	question:         "What's wrong with my app?",
	history:          nil,
	scenario:         drops.Definition, // drops.Definition or dns.Definition
	parameters: map[string]string{
		scenarios.Namespace1.Name: "default",
		// scenarios.PodPrefix1.Name: "toolbox-pod",
		// scenarios.Namespace2.Name: "default",
		// scenarios.PodPrefix2.Name: "toolbox-pod",
		// dns.DNSQuery.Name:         "google.com",
		// scenarios.Nodes.Name:      "[node1,node2]",
	},
}

func main() {
	run(defaultConfig)
}

func run(cfg *config) {
	log := logrus.New()
	// log.SetLevel(logrus.DebugLevel)

	log.Info("starting app...")

	// retrieve configs
	if cfg.kubeconfigPath == "" {
		usr, err := user.Current()
		if err != nil {
			log.WithError(err).Fatal("failed to get current user")
		}
		cfg.kubeconfigPath = usr.HomeDir + "/.kube/config"
	}

	kconfig, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfigPath)
	if err != nil {
		log.WithError(err).Fatal("failed to get kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		log.WithError(err).Fatal("failed to create clientset")
	}
	log.Info("retrieved kubeconfig and clientset")

	// configure LM (language model)
	var model lm.Model
	switch cfg.model {
	case "echo":
		model = lm.NewEchoModel()
		log.Info("initialized echo model")
	case "AOAI":
		model, err = lm.NewAzureOpenAI()
		if err != nil {
			log.WithError(err).Fatal("failed to create Azure OpenAI model")
		}
		log.Info("initialized Azure OpenAI model")
	default:
		log.Fatalf("unsupported model: %s", cfg.model)
	}

	bot := chat.NewBot(log, kconfig, clientset, model, cfg.useFlowsFromFile)
	newHistory, err := bot.HandleScenario(cfg.question, cfg.history, cfg.scenario, cfg.parameters)
	if err != nil {
		log.WithError(err).Fatal("error handling scenario")
	}

	log.Info("handled scenario")
	fmt.Println(newHistory[len(newHistory)-1].Assistant)
}
