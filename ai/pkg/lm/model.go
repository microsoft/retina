package lm

import "context"

type MessagePair struct {
	User      string
	Assistant string
}

type ChatHistory []MessagePair

type Model interface {
	Generate(ctx context.Context, systemPrompt string, history ChatHistory, message string) (string, error)
}
