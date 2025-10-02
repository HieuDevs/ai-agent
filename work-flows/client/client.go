package client

import "ai-agent/work-flows/models"

type Client interface {
	ChatCompletionStream(model string, temperature float64, maxTokens int, messages []models.Message, streamResponse chan<- models.StreamResponse, done chan<- bool)
}
