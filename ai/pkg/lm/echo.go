package lm

import (
	"context"
	"fmt"
	"strings"
)

// EchoModel is a mock model that echoes the prompt back
type EchoModel struct{}

func NewEchoModel() *EchoModel {
	return &EchoModel{}
}

func (m *EchoModel) Generate(ctx context.Context, systemPrompt string, chat ChatHistory, message string) (string, error) {
	chatStrings := make([]string, 0, len(chat))
	for _, pair := range chat {
		chatStrings = append(chatStrings, fmt.Sprintf("USER: %s\nASSISTANT: %s\n", pair.User, pair.Assistant))
	}
	resp := fmt.Sprintf("systemPrompt: %s\nhistory: %s\nmessage: %s", systemPrompt, strings.Join(chatStrings, "\n"), message)
	return resp, nil
}
