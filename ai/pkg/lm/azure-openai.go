package lm

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	endpointPattern   = `^https://[a-zA-Z0-9-]+\.openai\.azure\.com/?$`
	deploymentPattern = `^[a-zA-Z0-9-]+$`
)

var (
	endpointRegex    = regexp.MustCompile(endpointPattern)
	deploymentRegex  = regexp.MustCompile(deploymentPattern)
	ErrNoCompletions = fmt.Errorf("no completions returned")
	ErrNoMessage     = fmt.Errorf("no message included in completion")
)

type AzureOpenAI struct {
	modelDeployment string
	client          *azopenai.Client
}

func NewAzureOpenAI() (*AzureOpenAI, error) {
	aoai := &AzureOpenAI{}

	// Ex: "https://<your-azure-openai-host>.openai.azure.com"
	azureOpenAIEndpoint := os.Getenv("AOAI_COMPLETIONS_ENDPOINT")
	if azureOpenAIEndpoint == "" {
		return nil, fmt.Errorf("set endpoint with environment variable AOAI_COMPLETIONS_ENDPOINT")
	}
	if !endpointRegex.MatchString(azureOpenAIEndpoint) {
		return nil, fmt.Errorf("invalid Azure OpenAI endpoint. must follow pattern: %s", endpointPattern)
	}

	modelDeployment := os.Getenv("AOAI_DEPLOYMENT_NAME")
	if modelDeployment == "" {
		return nil, fmt.Errorf("set model deployment name with environment variable AOAI_DEPLOYMENT_NAME")
	}
	if !deploymentRegex.MatchString(modelDeployment) {
		return nil, fmt.Errorf("invalid Azure OpenAI deployment name. must follow pattern: %s", deploymentPattern)
	}
	aoai.modelDeployment = modelDeployment

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	// NOTE: this constructor creates a client that connects to an Azure OpenAI endpoint.
	// To connect to the public OpenAI endpoint, use azopenai.NewClientForOpenAI (requires an OpenAI API key).
	client, err := azopenai.NewClient(azureOpenAIEndpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %w", err)
	}
	aoai.client = client

	return aoai, nil
}

func (m *AzureOpenAI) Generate(ctx context.Context, systemPrompt string, chat ChatHistory, message string) (string, error) {
	messages := []azopenai.ChatRequestMessageClassification{
		&azopenai.ChatRequestSystemMessage{Content: to.Ptr(systemPrompt)},
	}
	for _, pair := range chat {
		messages = append(messages, &azopenai.ChatRequestUserMessage{Content: azopenai.NewChatRequestUserMessageContent(pair.User)})
		messages = append(messages, &azopenai.ChatRequestAssistantMessage{Content: to.Ptr(pair.Assistant)})
	}
	messages = append(messages, &azopenai.ChatRequestUserMessage{Content: azopenai.NewChatRequestUserMessageContent(message)})

	chatOptions := azopenai.ChatCompletionsOptions{
		Messages:       messages,
		MaxTokens:      to.Ptr(int32(2048)),
		N:              to.Ptr(int32(1)),
		Temperature:    to.Ptr(float32(0.0)),
		DeploymentName: &m.modelDeployment,
	}
	resp, err := m.client.GetChatCompletions(ctx, chatOptions, nil)

	if err != nil {
		return "", fmt.Errorf("failed to get completions: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", ErrNoCompletions
	}

	choice := resp.Choices[0]
	if choice.Message == nil || choice.Message.Content == nil {
		return "", ErrNoMessage
	}

	// TODO check ContentFilterResultsForChoice. And CompletionsFinishReason?
	return *choice.Message.Content, nil
}
