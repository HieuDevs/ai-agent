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

func (oc *openRouterClient) ChatCompletionStream(model string, temperature float64, maxTokens int, messages []models.Message, streamResponse chan<- models.StreamResponse, done chan<- bool) {
	defer func() { done <- true }()

	reqBody := models.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
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

func (oc *openRouterClient) ChatCompletion(model string, temperature float64, maxTokens int, messages []models.Message) (string, error) {
	reqBody := models.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", oc.baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", ContentTypeHeader)

	resp, err := oc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var chatResp models.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (oc *openRouterClient) ChatCompletionWithFormat(model string, temperature float64, maxTokens int, messages []models.Message, responseFormat *models.ResponseFormat) (string, error) {
	reqBody := models.ChatRequest{
		Model:          model,
		Messages:       messages,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		Stream:         false,
		ResponseFormat: responseFormat,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", oc.baseURL+"/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+oc.apiKey)
	req.Header.Set("Content-Type", ContentTypeHeader)

	resp, err := oc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var chatResp models.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (oc *openRouterClient) ChatCompletionWithFormatStream(model string, temperature float64, maxTokens int, messages []models.Message, responseFormat *models.ResponseFormat, streamResponse chan<- models.StreamResponse, done chan<- bool) {
	defer func() { done <- true }()

	reqBody := models.ChatRequest{
		Model:          model,
		Messages:       messages,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		Stream:         true,
		ResponseFormat: responseFormat,
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
