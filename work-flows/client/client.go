package client

import "ai-agent/work-flows/models"

type Client interface {
	ChatCompletion(model string, temperature float64, maxTokens int, messages []models.Message) (string, error)
	ChatCompletionStream(model string, temperature float64, maxTokens int, messages []models.Message, streamResponse chan<- models.StreamResponse, done chan<- bool)
	ChatCompletionWithFormat(model string, temperature float64, maxTokens int, messages []models.Message, responseFormat *models.ResponseFormat) (string, error)
	ChatCompletionWithFormatStream(model string, temperature float64, maxTokens int, messages []models.Message, responseFormat *models.ResponseFormat, streamResponse chan<- models.StreamResponse, done chan<- bool)
}
