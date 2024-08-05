package main

import (
	"github.com/microsoft/retina/ai/pkg/chat"
	"github.com/microsoft/retina/ai/pkg/lm"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeconfigPath = "/home/hunter/.kube/config"

// const kubeconfigPath = "C:\\Users\\hgregory\\.kube\\config"

func main() {
	log := logrus.New()
	// log.SetLevel(logrus.DebugLevel)

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
	// log.Info("initialized echo model")
	model, err := lm.NewAzureOpenAI()
	if err != nil {
		log.WithError(err).Fatal("failed to create Azure OpenAI model")
	}
	log.Info("initialized Azure OpenAI model")

	bot := chat.NewBot(log, config, clientset, model)
	if err := bot.Loop(); err != nil {
		log.WithError(err).Fatal("error running chat loop")
	}
}
