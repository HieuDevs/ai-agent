package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ai-agent/work-flows/models"
)

const (
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	ContentTypeHeader = "application/json"
)

type openRouterClient struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

func NewOpenRouterClient(apiKey string) *openRouterClient {
	return &openRouterClient{
		apiKey:  apiKey,
		client:  &http.Client{},
		baseURL: OpenRouterBaseURL,
	}
}

func (oc *openRouterClient) ChatCompletionStream(model string, messages []models.Message, streamResponse chan<- models.StreamResponse, done chan<- bool) {
	defer func() { done <- true }()

	reqBody := models.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		streamResponse <- models.StreamResponse{
			Error: err.Error(),
		}
		return
	}

	req, err := http.NewRequest("POST", oc.baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		streamResponse <- models.StreamResponse{
			Error: err.Error(),
		}
		return
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", ContentTypeHeader)

	resp, err := oc.client.Do(req)
	if err != nil {
		streamResponse <- models.StreamResponse{
			Error: err.Error(),
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		streamResponse <- models.StreamResponse{
			Error: fmt.Sprintf("Error: API request failed with status %d", resp.StatusCode),
		}
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data := strings.TrimSpace(after)

			if data == "[DONE]" {
				break
			}

			var streamResp models.StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
				streamResponse <- streamResp
			}
		}
	}

	if err := scanner.Err(); err != nil {
		streamResponse <- models.StreamResponse{
			Error: fmt.Sprintf("Error reading response: %s", err.Error()),
		}
	}
}
