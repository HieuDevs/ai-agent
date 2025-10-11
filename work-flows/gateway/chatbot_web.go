package gateway

import (
	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/managers"
	"ai-agent/work-flows/models"
	"ai-agent/work-flows/services"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ChatbotWeb struct {
	sessions map[string]*managers.AgentManager
	mu       sync.Mutex
	apiKey   string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Message   string `json:"message"`
	Action    string `json:"action"`
	Topic     string `json:"topic,omitzero"`
	Level     string `json:"level,omitzero"`
	Language  string `json:"language,omitzero"`
	SessionID string `json:"session_id,omitzero"`
}

type ChatResponse struct {
	Success     bool          `json:"success"`
	Message     string        `json:"message,omitzero"`
	Stats       any           `json:"stats,omitzero"`
	Level       string        `json:"level,omitzero"`
	Topic       string        `json:"topic,omitzero"`
	Topics      []string      `json:"topics,omitzero"`
	History     []ChatMessage `json:"history,omitzero"`
	Prompts     []PromptInfo  `json:"prompts,omitzero"`
	Content     string        `json:"content,omitzero"`
	Evaluation  any           `json:"evaluation,omitzero"`
	Suggestions any           `json:"suggestions,omitzero"`
	SessionID   string        `json:"session_id,omitzero"`
}

type PromptInfo struct {
	Name    string `json:"name"`
	Topic   string `json:"topic"`
	Content string `json:"content,omitzero"`
}

func NewChatbotWeb(apiKey string) *ChatbotWeb {
	web := &ChatbotWeb{
		sessions: make(map[string]*managers.AgentManager),
		apiKey:   apiKey,
	}

	return web
}

func (cw *ChatbotWeb) StartWebServer(port string) {

	http.HandleFunc("/", cw.serveChatHTML)
	// Orchestrator
	http.HandleFunc("/api/create-session", cw.handleCreateSession)
	http.HandleFunc("/api/stream", cw.handleStream)
	http.HandleFunc("/api/translate", cw.handleTranslate)
	http.HandleFunc("/api/suggestions", cw.handleGetSuggestions)
	http.HandleFunc("/api/assessment", cw.handleGetAssessmentStream)
	// Prompts + Topics
	http.HandleFunc("/api/prompts", cw.handleGetPrompts)
	http.HandleFunc("/api/topics", cw.handleGetTopics)
	http.HandleFunc("/api/prompt/content", cw.handleGetPromptContent)
	http.HandleFunc("/api/prompt/save", cw.handleSavePrompt)
	http.HandleFunc("/api/prompt/create", cw.handleCreatePrompt)
	http.HandleFunc("/api/prompt/delete", cw.handleDeletePrompt)

	addr := ":" + port
	fmt.Printf("🌐 Web server starting at http://localhost%s\n", addr)
	fmt.Printf("📱 Open your browser and navigate to the URL above\n\n")

	log.Fatal(http.ListenAndServe(addr, nil))
}

func (cw *ChatbotWeb) handleStream(w http.ResponseWriter, r *http.Request) {
	userMessage := r.URL.Query().Get("message")
	sessionID := r.URL.Query().Get("session_id")
	if userMessage == "" {
		http.Error(w, "No message provided", http.StatusBadRequest)
		return
	}
	if sessionID == "" {
		http.Error(w, "No session ID provided", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	cw.mu.Lock()

	manager, exists := cw.sessions[sessionID]
	if !exists {
		cw.mu.Unlock()
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	conversationLevel := manager.GetConversationAgent().GetLevel()
	pathPrompts := filepath.Join(utils.GetPromptsDir(), manager.GetConversationAgent().Topic+"_prompt.yaml")
	levelPrompt := agents.GetLevelSpecificPrompt(pathPrompts, conversationLevel, "conversational")

	messages := []models.Message{
		{
			Role:    models.MessageRoleSystem,
			Content: levelPrompt,
		},
	}
	history := manager.GetHistoryManager().GetConversationHistory()
	if len(history) > 0 {
		messages = append(messages, history...)
	}

	messages = append(messages, models.Message{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	})

	streamResponseChan := make(chan models.StreamResponse, 10)
	done := make(chan bool)
	evaluationChan := make(chan map[string]any, 1)

	// Record user's message first
	manager.GetHistoryManager().AddMessage(models.MessageRoleUser, userMessage)
	// manager.GetHistoryManager().EnforceMax(20)

	// Run evaluation in parallel (non-blocking) and attach to the exact user message index
	evaluateAgent, evalExists := manager.GetAgent("EvaluateAgent")
	if evalExists {
		go func() {
			defer close(evaluationChan)

			lastAIMessage := ""
			history := manager.GetHistoryManager().GetConversationHistory()
			for i := len(history) - 1; i >= 0; i-- {
				if history[i].Role == models.MessageRoleAssistant {
					lastAIMessage = history[i].Content
					break
				}
			}

			utils.PrintInfo(fmt.Sprintf("Evaluating user message: '%s', Last AI: '%s'", userMessage, lastAIMessage))

			evaluateJob := models.JobRequest{
				Task:          "evaluate",
				UserMessage:   userMessage,
				LastAIMessage: lastAIMessage,
			}
			evaluateResponse := evaluateAgent.ProcessTask(evaluateJob)
			if evaluateResponse.Success {
				utils.PrintInfo("Evaluation successful, preparing to send to client")
				var evaluationMap map[string]any
				if err := json.Unmarshal([]byte(evaluateResponse.Result), &evaluationMap); err == nil {
					utils.PrintInfo("Sending evaluation to channel")
					evaluationChan <- evaluationMap
					utils.PrintInfo("Evaluation sent to channel successfully")
					// Attach parsed evaluation to the most recent user message
					if parsed, err := agents.ParseEvaluationResponse(evaluateResponse.Result); err == nil {
						manager.GetHistoryManager().UpdateLastEvaluation(parsed)
					}
				} else {
					utils.PrintError(fmt.Sprintf("Failed to unmarshal evaluation: %v", err))
				}
			} else {
				utils.PrintError(fmt.Sprintf("Evaluation failed: %s", evaluateResponse.Error))
			}
		}()
	} else {
		utils.PrintInfo("EvaluateAgent not found, skipping evaluation")
		close(evaluationChan)
	}

	go manager.GetConversationAgent().GetClient().ChatCompletionStream(
		manager.GetConversationAgent().GetModel(),
		manager.GetConversationAgent().GetTemperature(),
		manager.GetConversationAgent().GetMaxTokens(),
		messages,
		streamResponseChan,
		done,
	)

	var fullResponse strings.Builder
	evaluationSent := false
	historyManager := manager.GetHistoryManager()

	for {
		select {
		case <-done:
			aiResponse := fullResponse.String()
			// Update the most recent AI message or create new one if none exists
			historyManager.UpdateLastMessage(models.MessageRoleAssistant, aiResponse)

			// Generate suggestions for the AI message and attach to most recent AI message
			if suggestionAgent, ok := manager.GetAgent("SuggestionAgent"); ok {
				suggestionJob := models.JobRequest{Task: "suggestion", LastAIMessage: aiResponse}
				suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
				if suggestionResponse.Success {
					var suggestion models.SuggestionResponse
					if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestion); err == nil {
						historyManager.UpdateLastSuggestion(&suggestion)
					}
				}
			}

			// Wait for evaluation if not yet received
			if !evaluationSent {
				utils.PrintInfo("Waiting for evaluation before sending done...")
				evalMap, ok := <-evaluationChan
				if ok && evalMap != nil {
					utils.PrintInfo("Received evaluation in done handler, sending to client")
					evalData := map[string]any{
						"done": false,
						"type": "evaluation",
						"data": evalMap,
					}
					evalJSON, _ := json.Marshal(evalData)
					fmt.Fprintf(w, "data: %s\n\n", evalJSON)
					flusher.Flush()
					evaluationSent = true
				} else {
					utils.PrintInfo("Evaluation channel closed in done handler")
					evaluationSent = true
				}
			}

			// Send final done event
			doneData := map[string]any{
				"done": true,
				"type": "done",
			}
			doneJSON, _ := json.Marshal(doneData)
			fmt.Fprintf(w, "data: %s\n\n", doneJSON)
			flusher.Flush()
			cw.mu.Unlock()
			return

		case evalMap, ok := <-evaluationChan:
			if ok && evalMap != nil && !evaluationSent {
				utils.PrintInfo("Sending evaluation to client via SSE")
				evalData := map[string]any{
					"done": false,
					"type": "evaluation",
					"data": evalMap,
				}
				evalJSON, _ := json.Marshal(evalData)
				utils.PrintInfo(fmt.Sprintf("Evaluation JSON: %s", string(evalJSON)))
				fmt.Fprintf(w, "data: %s\n\n", evalJSON)
				flusher.Flush()
				evaluationSent = true
			} else if !ok {
				utils.PrintInfo("Evaluation channel closed without data")
			}

		case streamResponse := <-streamResponseChan:
			if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.Content != "" {
				content := streamResponse.Choices[0].Delta.Content
				fullResponse.WriteString(content)

				data := map[string]any{
					"content": content,
					"done":    false,
					"type":    "message",
				}
				jsonData, _ := json.Marshal(data)
				fmt.Fprintf(w, "data: %s\n\n", jsonData)
				flusher.Flush()
			}
		}
	}
}

func (cw *ChatbotWeb) handleGetTopics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	topics := getAvailableTopics()

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Topics:  topics,
	})
}

func (cw *ChatbotWeb) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Topic == "" || req.Level == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic and level are required",
		})
		return
	}

	level := models.ConversationLevel(req.Level)
	if !models.IsValidConversationLevel(string(level)) {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid level",
		})
		return
	}

	userLanguage := req.Language
	if userLanguage == "" {
		userLanguage = "Vietnamese"
	}

	cw.mu.Lock()
	var sessionID string
	if req.SessionID != "" {
		sessionID = req.SessionID
		// If session exists, remove it to create a new one
		delete(cw.sessions, sessionID)
	} else {
		sessionID = fmt.Sprintf("web_%d", utils.GetCurrentTimestamp())
	}

	manager := managers.NewManager(cw.apiKey, level, req.Topic, userLanguage, sessionID)
	cw.sessions[sessionID] = manager
	cw.mu.Unlock()

	conversationJob := models.JobRequest{
		Task: "conversation",
	}
	response := manager.ProcessJob(conversationJob)

	conversationAgent := manager.GetConversationAgent()
	stats := manager.GetHistoryManager().GetConversationStats()

	json.NewEncoder(w).Encode(ChatResponse{
		Success:   response.Success,
		Message:   response.Result,
		Stats:     stats,
		Level:     string(conversationAgent.GetLevel()),
		Topic:     cases.Title(language.English).String(conversationAgent.Topic),
		SessionID: sessionID,
	})
}

func getAvailableTopics() []string {
	configDir := utils.GetPromptsDir()
	files, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		log.Printf("Error reading config directory: %v", err)
		return []string{"sports"}
	}

	var topics []string
	for _, file := range files {
		filename := filepath.Base(file)
		if strings.HasPrefix(filename, "_") {
			continue
		}
		if strings.HasSuffix(filename, "_prompt.yaml") {
			topic := strings.TrimSuffix(filename, "_prompt.yaml")
			if topic != "" {
				topics = append(topics, topic)
			}
		}
	}

	return topics
}

func (cw *ChatbotWeb) handleGetPrompts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	configDir := utils.GetPromptsDir()
	files, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to read prompts directory",
		})
		return
	}

	var prompts []PromptInfo
	for _, file := range files {
		filename := filepath.Base(file)
		if strings.HasSuffix(filename, "_prompt.yaml") {
			topic := strings.TrimSuffix(filename, "_prompt.yaml")
			if topic != "" {
				prompts = append(prompts, PromptInfo{
					Name:  filename,
					Topic: topic,
				})
			}
		}
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Prompts: prompts,
	})
}

func (cw *ChatbotWeb) handleGetPromptContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic is required",
		})
		return
	}

	promptPath := filepath.Join(utils.GetPromptsDir(), topic+"_prompt.yaml")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to read prompt file",
		})
		return
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Content: string(content),
	})
}

func (cw *ChatbotWeb) handleSavePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Topic   string `json:"topic"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Topic == "" || req.Content == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic and content are required",
		})
		return
	}

	promptPath := filepath.Join(utils.GetPromptsDir(), req.Topic+"_prompt.yaml")
	if err := os.WriteFile(promptPath, []byte(req.Content), 0644); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to save prompt file",
		})
		return
	}

	// Clear prompt caches to reload updated configuration
	if strings.HasPrefix(req.Topic, "_") {
		// System prompt - clear specific cache based on topic
		switch req.Topic {
		case "_suggestion_vocab":
			utils.ClearSuggestionPromptCache()
		case "_evaluate":
			utils.ClearEvaluatePromptCache()
		case "_assessment":
			utils.ClearAssessmentPromptCache()
		default:
			// For other system prompts, clear all caches to be safe
			utils.ClearAllPromptCaches()
		}
	} else {
		// Regular conversation prompt - clear conversation cache for this topic
		utils.ClearConversationPromptCache()
	}

	shouldReset := false
	cw.mu.Lock()
	for _, manager := range cw.sessions {
		conversationAgent := manager.GetConversationAgent()
		if conversationAgent.Topic == req.Topic {
			shouldReset = true
			manager.GetHistoryManager().ResetConversation()
		}
	}
	cw.mu.Unlock()

	message := "Prompt saved successfully"
	if shouldReset {
		message = "Prompt saved and conversation reset"
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: message,
	})
}

func (cw *ChatbotWeb) handleCreatePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Topic   string `json:"topic"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Topic == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic name is required",
		})
		return
	}

	promptPath := filepath.Join(utils.GetPromptsDir(), req.Topic+"_prompt.yaml")

	if _, err := os.Stat(promptPath); err == nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Prompt file already exists",
		})
		return
	}

	content := req.Content
	if content == "" {
		content = `conversation_levels:

  beginner:
    role: "Friendly conversation partner"
    personality: "Warm, encouraging, and genuinely interested"
    llm:
      model: "openai/gpt-4o-mini"
      temperature: 0.2
      max_tokens: 250
    starter: |
      Hi! Let's talk about ` + req.Topic + `!
    conversational: |
      Have natural, friendly conversations:
      - Respond naturally to what they say
      - Share your own thoughts and experiences
      - Ask follow-up questions to keep the conversation flowing
      - Show genuine interest in their responses
      - Keep responses simple and friendly

  intermediate:
    role: "Engaging conversation partner"
    personality: "Thoughtful, curious, and naturally expressive"
    llm:
      model: "openai/gpt-4o-mini"
      temperature: 0.2
      max_tokens: 250
    starter: |
      What interests you most about ` + req.Topic + `?
    conversational: |
      Have meaningful conversations:
      - Respond thoughtfully to their ideas
      - Share deeper insights and personal experiences
      - Ask questions that explore their perspectives
      - Express your own opinions and views
      - Keep the dialogue interesting and engaging
`
	}

	if err := os.WriteFile(promptPath, []byte(content), 0644); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to create prompt file",
		})
		return
	}

	// Clear prompt caches to reload updated configuration
	if strings.HasPrefix(req.Topic, "_") {
		// System prompt - clear specific cache based on topic
		switch req.Topic {
		case "_suggestion_vocab":
			utils.ClearSuggestionPromptCache()
		case "_evaluate":
			utils.ClearEvaluatePromptCache()
		case "_assessment":
			utils.ClearAssessmentPromptCache()
		default:
			// For other system prompts, clear all caches to be safe
			utils.ClearAllPromptCaches()
		}
	} else {
		// Regular conversation prompt - clear conversation cache for this topic
		utils.ClearConversationPromptCache()
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Prompt file created successfully",
		Topic:   req.Topic,
	})
}

func (cw *ChatbotWeb) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Topic string `json:"topic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Topic == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic name is required",
		})
		return
	}

	promptPath := filepath.Join(utils.GetPromptsDir(), req.Topic+"_prompt.yaml")

	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Prompt file not found",
		})
		return
	}

	if err := os.Remove(promptPath); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to delete prompt file",
		})
		return
	}

	// Clear prompt caches to reload updated configuration
	if strings.HasPrefix(req.Topic, "_") {
		// System prompt - clear specific cache based on topic
		switch req.Topic {
		case "_suggestion_vocab":
			utils.ClearSuggestionPromptCache()
		case "_evaluate":
			utils.ClearEvaluatePromptCache()
		case "_assessment":
			utils.ClearAssessmentPromptCache()
		default:
			// For other system prompts, clear all caches to be safe
			utils.ClearAllPromptCaches()
		}
	} else {
		// Regular conversation prompt - clear conversation cache for this topic
		utils.ClearConversationPromptCache()
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Prompt file deleted successfully",
	})
}

func (cw *ChatbotWeb) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Text == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: true,
			Content: "",
		})
		return
	}

	translated, err := services.TranslateToVietnamese(req.Text)
	if err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Translation failed",
		})
		return
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Content: translated,
	})
}

func (cw *ChatbotWeb) handleGetSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Message == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Message is required",
		})
		return
	}

	if req.SessionID == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Session ID is required",
		})
		return
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	manager, exists := cw.sessions[req.SessionID]
	if !exists {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid session ID",
		})
		return
	}

	suggestionAgent, exists := manager.GetAgent("SuggestionAgent")
	if !exists {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Suggestion agent not available",
		})
		return
	}

	suggestionJob := models.JobRequest{
		Task:          "suggestion",
		LastAIMessage: req.Message,
	}

	suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
	if !suggestionResponse.Success {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to get suggestions",
		})
		return
	}

	var suggestionsMap map[string]any
	if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestionsMap); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Failed to parse suggestions",
		})
		return
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success:     true,
		Suggestions: suggestionsMap,
	})
}

func (cw *ChatbotWeb) handleGetAssessmentStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	manager, exists := cw.sessions[sessionID]
	if !exists {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	assessmentAgent, exists := manager.GetAgent("AssessmentAgent")
	if !exists {
		http.Error(w, "Assessment agent not available", http.StatusBadRequest)
		return
	}

	historyManager := manager.GetHistoryManager()
	if historyManager.Len() == 0 {
		errorData := map[string]any{
			"done":  true,
			"type":  "error",
			"error": "No conversation history available for assessment",
		}
		errorJSON, _ := json.Marshal(errorData)
		fmt.Fprintf(w, "data: %s\n\n", errorJSON)
		flusher.Flush()
		return
	}

	// Create progress channel
	progressChan := make(chan models.AssessmentStreamResponse, 100)

	// Start streaming assessment
	go func() {
		if aa, ok := assessmentAgent.(*agents.AssessmentAgent); ok {
			aa.GenerateAssessmentStream(historyManager, progressChan)
		} else {
			progressChan <- models.AssessmentStreamResponse{
				Error: "Assessment agent type assertion failed",
			}
		}
	}()

	// Handle progress events
	for response := range progressChan {
		if response.Error != "" {
			errorData := map[string]any{
				"done":  true,
				"type":  "error",
				"error": response.Error,
			}
			errorJSON, _ := json.Marshal(errorData)
			fmt.Fprintf(w, "data: %s\n\n", errorJSON)
			flusher.Flush()
			return
		}

		if response.ProgressEvent != nil {
			event := response.ProgressEvent
			progressData := map[string]any{
				"done": false,
				"type": "progress",
				"data": map[string]any{
					"type":        event.Type,
					"message":     event.Message,
					"progress":    event.Progress,
					"is_complete": event.IsComplete,
				},
			}
			progressJSON, _ := json.Marshal(progressData)
			fmt.Fprintf(w, "data: %s\n\n", progressJSON)
			flusher.Flush()
		}

		if response.FinalResult != "" {
			// Parse and send final assessment result
			var assessmentMap map[string]any
			if err := json.Unmarshal([]byte(response.FinalResult), &assessmentMap); err == nil {
				finalData := map[string]any{
					"done":       true,
					"type":       "assessment",
					"assessment": assessmentMap,
				}
				finalJSON, _ := json.Marshal(finalData)
				fmt.Fprintf(w, "data: %s\n\n", finalJSON)
				flusher.Flush()
			} else {
				errorData := map[string]any{
					"done":  true,
					"type":  "error",
					"error": "Failed to parse assessment result",
				}
				errorJSON, _ := json.Marshal(errorData)
				fmt.Fprintf(w, "data: %s\n\n", errorJSON)
				flusher.Flush()
			}
			break
		}
	}
}

func (cw *ChatbotWeb) serveChatHTML(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>English Conversation Chatbot</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #f5f5f5;
            height: 100vh;
            display: flex;
            overflow: hidden;
        }
        
        .sidebar {
            width: 320px;
            background: white;
            border-right: 1px solid #e0e0e0;
            display: flex;
            flex-direction: column;
            overflow-y: auto;
        }
        
        .sidebar-header {
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        
        .sidebar-header h2 {
            font-size: 20px;
            margin-bottom: 5px;
        }
        
        .sidebar-header p {
            font-size: 13px;
            opacity: 0.9;
        }
        
        .sidebar-content {
            padding: 20px;
            flex: 1;
        }
        
        .section {
            margin-bottom: 25px;
        }
        
        .section-title {
            font-size: 14px;
            font-weight: 600;
            color: #333;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        .form-select {
            width: 100%;
            padding: 10px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 14px;
            outline: none;
            background: white;
            cursor: pointer;
        }
        
        .form-select:focus {
            border-color: #667eea;
        }
        
        .level-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 8px;
        }
        
        .level-option {
            padding: 10px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.2s;
            text-align: center;
            font-size: 12px;
        }
        
        .level-option:hover {
            border-color: #667eea;
            background: #f8f9ff;
        }
        
        .level-option.selected {
            border-color: #667eea;
            background: #667eea;
            color: white;
        }
        
        .level-option-title {
            font-weight: 600;
        }
        
        .prompt-list {
            max-height: 200px;
            overflow-y: auto;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
        }
        
        .prompt-item {
            padding: 12px;
            border-bottom: 1px solid #e0e0e0;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .prompt-item:last-child {
            border-bottom: none;
        }
        
        .prompt-item:hover {
            background: #f8f9ff;
        }
        
        .prompt-name {
            font-size: 13px;
            color: #333;
            flex: 1;
        }
        
        .prompt-actions {
            display: flex;
            gap: 5px;
        }
        
        .btn-edit, .btn-delete {
            padding: 5px 12px;
            color: white;
            border: none;
            border-radius: 5px;
            cursor: pointer;
            font-size: 11px;
        }
        
        .btn-edit {
            background: #667eea;
        }
        
        .btn-edit:hover {
            background: #5568d3;
        }
        
        .btn-delete {
            background: #f44336;
        }
        
        .btn-delete:hover {
            background: #d32f2f;
        }
        
        .chat-container {
            flex: 1;
            display: flex;
            flex-direction: column;
            background: white;
        }
        
        .chat-header {
            padding: 20px;
            background: white;
            border-bottom: 1px solid #e0e0e0;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .chat-title {
            font-size: 18px;
            font-weight: 600;
            color: #333;
        }
        
        .chat-info {
            font-size: 13px;
            color: #666;
        }
        
        .chat-messages {
            flex: 1;
            overflow-y: auto;
            padding: 20px;
            background: #f9fafb;
        }
        
        .message {
            margin-bottom: 16px;
            animation: fadeIn 0.3s;
        }
        
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }
        
        .message.user {
            display: flex;
            flex-direction: column;
            align-items: flex-end;
        }
        
        .message.assistant {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
        }
        
        .message-content {
            max-width: 70%;
            padding: 12px 16px;
            border-radius: 12px;
            font-size: 14px;
            line-height: 1.5;
        }
        
        .message.assistant .message-content {
            background: white;
            color: #333;
            border: 1px solid #e0e0e0;
        }
        
        .message.user .message-content {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        
        .message-translation {
            max-width: 70%;
            padding: 8px 12px;
            margin-top: 8px;
            font-size: 13px;
            color: #666;
            background: #f5f5f5;
            border-radius: 8px;
            font-style: italic;
            border-left: 3px solid #667eea;
        }
        
        .translation-loading {
            color: #999;
            font-size: 12px;
        }

        .message-evaluation {
            max-width: 70%;
            margin-top: 12px;
            background: #e3f2fd;
            border-radius: 8px;
            overflow: hidden;
            border: 1px solid #bbdefb;
        }

        .evaluation-header {
            padding: 8px 12px;
            background: #bbdefb;
            color: #1565c0;
            font-weight: 600;
            font-size: 13px;
        }

        .evaluation-content {
            padding: 12px;
            font-size: 13px;
            color: #333;
            line-height: 1.5;
        }

        .evaluation-score {
            margin-top: 8px;
            font-weight: 600;
            color: #1565c0;
        }

        .message-suggestions {
            max-width: 70%;
            margin-top: 12px;
            background: #e8f5e9;
            border-radius: 8px;
            overflow: hidden;
            border: 1px solid #c8e6c9;
        }

        .suggestions-header {
            padding: 8px 12px;
            background: #c8e6c9;
            color: #2e7d32;
            font-weight: 600;
            font-size: 13px;
        }

        .suggestions-content {
            padding: 12px;
        }

        .suggestion-lead {
            font-size: 14px;
            color: #333;
            margin-bottom: 10px;
            line-height: 1.5;
        }

        .suggestion-options {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .suggestion-option {
            padding: 8px 12px;
            background: white;
            border: 1px solid #c8e6c9;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            color: #333;
            transition: all 0.2s;
        }

        .suggestion-option:hover {
            background: #c8e6c9;
            border-color: #81c784;
            transform: translateY(-1px);
        }
        
        .typing-indicator {
            display: flex;
            align-items: center;
            gap: 5px;
            padding: 12px 16px;
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 12px;
            max-width: 70px;
        }
        
        .typing-indicator span {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: #667eea;
            animation: bounce 1.4s infinite;
        }
        
        .typing-indicator span:nth-child(2) {
            animation-delay: 0.2s;
        }
        
        .typing-indicator span:nth-child(3) {
            animation-delay: 0.4s;
        }
        
        @keyframes bounce {
            0%, 60%, 100% { transform: translateY(0); }
            30% { transform: translateY(-10px); }
        }
        
        .chat-input-container {
            padding: 20px;
            background: white;
            border-top: 1px solid #e0e0e0;
        }
        
        .chat-input-wrapper {
            display: flex;
            gap: 10px;
            align-items: flex-end;
        }
        
        .chat-input {
            flex: 1;
            padding: 12px 16px;
            border: 2px solid #e0e0e0;
            border-radius: 10px;
            font-size: 14px;
            outline: none;
            resize: none;
            font-family: inherit;
        }
        
        .chat-input:focus {
            border-color: #667eea;
        }
        
        .btn-send {
            padding: 12px 24px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            font-weight: 600;
            font-size: 14px;
        }
        
        .btn-send:hover:not(:disabled) {
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        
        .btn-send:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        
        .btn-hint {
            padding: 12px 24px;
            background: #4CAF50;
            color: white;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            font-weight: 600;
            font-size: 14px;
        }
        
        .btn-hint:hover:not(:disabled) {
            background: #45a049;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(76, 175, 80, 0.4);
        }
        
        .btn-hint:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        
        .btn-assessment {
            padding: 12px 24px;
            background: #FF9800;
            color: white;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            font-weight: 600;
            font-size: 14px;
        }
        
        .btn-assessment:hover:not(:disabled) {
            background: #F57C00;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(255, 152, 0, 0.4);
        }
        
        .btn-assessment:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }
        
        .modal.active {
            display: flex;
        }
        
        .modal-content {
            background: white;
            border-radius: 12px;
            width: 90%;
            max-width: 800px;
            max-height: 90vh;
            display: flex;
            flex-direction: column;
        }
        
        .modal-header {
            padding: 20px;
            border-bottom: 1px solid #e0e0e0;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .modal-title {
            font-size: 18px;
            font-weight: 600;
        }
        
        .btn-close {
            background: none;
            border: none;
            font-size: 24px;
            cursor: pointer;
            color: #666;
        }
        
        .modal-body {
            padding: 20px;
            flex: 1;
            overflow-y: auto;
        }
        
        .modal-footer {
            padding: 20px;
            border-top: 1px solid #e0e0e0;
            display: flex;
            justify-content: flex-end;
            gap: 10px;
        }
        
        .btn-secondary {
            padding: 10px 20px;
            background: #e0e0e0;
            color: #333;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
        }
        
        .btn-primary {
            padding: 10px 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
        }
        
        .prompt-editor {
            width: 100%;
            min-height: 400px;
            padding: 12px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-family: 'Courier New', monospace;
            font-size: 15px;
            line-height: 1.6;
            outline: none;
            resize: vertical;
        }
        
        .prompt-editor:focus {
            border-color: #667eea;
        }
        
        .prompt-editor.error {
            border-color: #f44336;
        }
        
        .yaml-error {
            color: #f44336;
            font-size: 13px;
            margin-top: 10px;
            padding: 10px;
            background: #ffebee;
            border-radius: 5px;
            display: none;
        }
        
        .yaml-error.active {
            display: block;
        }
        
        .assessment-content {
            max-height: 60vh;
            overflow-y: auto;
            padding: 20px;
            background: #f9fafb;
            border-radius: 8px;
            margin: 20px 0;
        }
        
        .assessment-section {
            margin-bottom: 20px;
            padding: 15px;
            background: white;
            border-radius: 8px;
            border-left: 4px solid #FF9800;
        }
        
        .assessment-section h3 {
            margin: 0 0 10px 0;
            color: #FF9800;
            font-size: 16px;
        }
        
        .assessment-level {
            font-size: 18px;
            font-weight: bold;
            color: #FF9800;
            margin-bottom: 10px;
        }
        
        .assessment-tip {
            margin: 8px 0;
            padding: 8px;
            background: #f0f0f0;
            border-radius: 4px;
            font-size: 14px;
        }
        
        .assessment-vocab {
            display: inline-block;
            margin: 2px 4px;
            padding: 4px 8px;
            background: #e3f2fd;
            border-radius: 4px;
            font-size: 12px;
            color: #1565c0;
        }
        
        .btn-add-prompt {
            width: 100%;
            padding: 10px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            font-size: 13px;
            margin-top: 10px;
        }
        
        .btn-add-prompt:hover {
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        
        .input-topic-name {
            width: 100%;
            padding: 10px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 14px;
            outline: none;
            margin-bottom: 15px;
        }
        
        .input-topic-name:focus {
            border-color: #667eea;
        }
        
        .notification {
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 15px 20px;
            background: #4CAF50;
            color: white;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 2000;
            display: none;
            animation: slideIn 0.3s;
        }
        
        .notification.active {
            display: block;
        }
        
        .notification.error {
            background: #f44336;
        }
        
        @keyframes slideIn {
            from { transform: translateX(100%); }
            to { transform: translateX(0); }
        }
        
        @media (max-width: 768px) {
            .sidebar {
                width: 280px;
            }
            
            .level-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="sidebar">
        <div class="sidebar-header">
            <h2>🎯 Chat Settings</h2>
            <p>Configure your conversation</p>
        </div>
        <div class="sidebar-content">
            <div class="section">
                <div class="section-title">Topic</div>
                <select id="topicSelect" class="form-select">
                    <option value="">Loading...</option>
                </select>
            </div>
            
            <div class="section">
                <div class="section-title">Level</div>
                <div class="level-grid" id="levelGrid">
                    <div class="level-option" data-level="beginner">
                        <div class="level-option-title">Beginner</div>
                    </div>
                    <div class="level-option" data-level="elementary">
                        <div class="level-option-title">Elementary</div>
                    </div>
                    <div class="level-option" data-level="intermediate">
                        <div class="level-option-title">Intermediate</div>
                    </div>
                    <div class="level-option" data-level="upper_intermediate">
                        <div class="level-option-title">Upper Int.</div>
                    </div>
                    <div class="level-option" data-level="advanced">
                        <div class="level-option-title">Advanced</div>
                    </div>
                    <div class="level-option" data-level="fluent">
                        <div class="level-option-title">Fluent</div>
                    </div>
                </div>
            </div>
            
            <div class="section">
                <div class="section-title">Prompt Files</div>
                <div class="prompt-list" id="promptList">
                    <div style="padding: 20px; text-align: center; color: #999;">Loading...</div>
                </div>
                <button class="btn-add-prompt" onclick="openNewPromptDialog()">+ Add New Prompt</button>
            </div>
        </div>
    </div>
    
    <div class="chat-container">
        <div class="chat-header">
            <div>
                <div class="chat-title" id="chatTitle">English Conversation</div>
                <div class="chat-info" id="chatInfo">Select topic and level to begin</div>
            </div>
        </div>
        <div class="chat-messages" id="chatMessages"></div>
        <div class="chat-input-container">
            <div class="chat-input-wrapper">
                <textarea id="chatInput" class="chat-input" placeholder="Type your message..." rows="1"></textarea>
                <button id="hintBtn" class="btn-hint" disabled>💡 Hint</button>
                <button id="assessmentBtn" class="btn-assessment" disabled>📊 End Conversation</button>
                <button id="sendBtn" class="btn-send" disabled>Send</button>
            </div>
        </div>
    </div>
    
    <div id="promptModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <div class="modal-title" id="modalTitle">Edit Prompt</div>
                <button class="btn-close" onclick="closePromptEditor()">&times;</button>
            </div>
            <div class="modal-body">
                <div id="newPromptNameSection" style="display: none;">
                    <input type="text" id="newPromptName" class="input-topic-name" placeholder="Enter topic name (e.g., music, technology)">
                </div>
                <textarea id="promptEditor" class="prompt-editor"></textarea>
                <div id="yamlError" class="yaml-error"></div>
            </div>
            <div class="modal-footer">
                <button class="btn-secondary" onclick="closePromptEditor()">Cancel</button>
                <button class="btn-primary" id="savePromptBtn" onclick="savePrompt()">Apply</button>
            </div>
        </div>
    </div>
    
    <div id="assessmentModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <div class="modal-title">📊 Conversation Assessment</div>
                <button class="btn-close" onclick="closeAssessmentModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div id="assessmentContent" class="assessment-content">
                    <div style="text-align: center; padding: 40px;">
                        <div style="font-size: 48px; margin-bottom: 20px;">⏳</div>
                        <div>Generating assessment...</div>
                    </div>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn-secondary" onclick="closeAssessmentModal()">Close</button>
            </div>
        </div>
    </div>
    
    <div id="notification" class="notification"></div>

    <script src="https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js"></script>
    <script>
        let currentTopic = '';
        let currentLevel = 'intermediate';
        let sessionActive = false;
        let editingPromptTopic = '';
        let isCreatingNew = false;
        let yamlValidationTimeout = null;
        let currentSessionID = '';

        async function init() {
            await loadTopics();
            await loadPrompts();
            document.querySelector('[data-level="intermediate"]').classList.add('selected');
        }

        async function loadTopics() {
            try {
                const response = await fetch('/api/topics');
                const data = await response.json();
                
                if (data.success && data.topics && data.topics.length > 0) {
                    const select = document.getElementById('topicSelect');
                    select.innerHTML = '';
                    data.topics.forEach(topic => {
                        const option = document.createElement('option');
                        option.value = topic;
                        option.textContent = topic.charAt(0).toUpperCase() + topic.slice(1);
                        select.appendChild(option);
                    });
                    currentTopic = data.topics[0];
                    select.value = currentTopic;
                    await createSession();
                }
            } catch (error) {
                console.error('Error loading topics:', error);
            }
        }

        async function loadPrompts() {
            try {
                const response = await fetch('/api/prompts');
                const data = await response.json();
                
                if (data.success && data.prompts) {
                    const list = document.getElementById('promptList');
                    list.innerHTML = '';
                    data.prompts.forEach(prompt => {
                        const item = document.createElement('div');
                        item.className = 'prompt-item';
                        item.innerHTML = '<div class="prompt-name">' + prompt.name + '</div>' +
                            '<div class="prompt-actions">' +
                            '<button class="btn-edit" onclick="editPrompt(\'' + prompt.topic + '\')">Edit</button>' +
                            '<button class="btn-delete" onclick="deletePrompt(\'' + prompt.topic + '\')">Delete</button>' +
                            '</div>';
                        list.appendChild(item);
                    });
                }
            } catch (error) {
                console.error('Error loading prompts:', error);
            }
        }

        async function editPrompt(topic) {
            isCreatingNew = false;
            editingPromptTopic = topic;
            try {
                const response = await fetch('/api/prompt/content?topic=' + topic);
                const data = await response.json();
                
                if (data.success) {
                    document.getElementById('modalTitle').textContent = 'Edit ' + topic + ' Prompt';
                    document.getElementById('promptEditor').value = data.content;
                    document.getElementById('newPromptNameSection').style.display = 'none';
                    document.getElementById('savePromptBtn').textContent = 'Apply';
                    document.getElementById('yamlError').classList.remove('active');
                    document.getElementById('promptEditor').classList.remove('error');
                    document.getElementById('promptModal').classList.add('active');
                    validateYAML();
                }
            } catch (error) {
                console.error('Error loading prompt:', error);
                showNotification('Failed to load prompt', true);
            }
        }

        function openNewPromptDialog() {
            isCreatingNew = true;
            editingPromptTopic = '';
            document.getElementById('modalTitle').textContent = 'Create New Prompt';
            document.getElementById('newPromptNameSection').style.display = 'block';
            document.getElementById('newPromptName').value = '';
            document.getElementById('promptEditor').value = '';
            document.getElementById('savePromptBtn').textContent = 'Create';
            document.getElementById('yamlError').classList.remove('active');
            document.getElementById('promptEditor').classList.remove('error');
            document.getElementById('promptModal').classList.add('active');
        }

        function closePromptEditor() {
            document.getElementById('promptModal').classList.remove('active');
            if (yamlValidationTimeout) {
                clearTimeout(yamlValidationTimeout);
            }
        }

        document.getElementById('promptEditor').addEventListener('input', () => {
            if (yamlValidationTimeout) {
                clearTimeout(yamlValidationTimeout);
            }
            yamlValidationTimeout = setTimeout(validateYAML, 500);
        });

        function validateYAML() {
            const content = document.getElementById('promptEditor').value;
            const errorDiv = document.getElementById('yamlError');
            const editor = document.getElementById('promptEditor');
            
            if (!content.trim()) {
                errorDiv.classList.remove('active');
                editor.classList.remove('error');
                return true;
            }

            try {
                jsyaml.load(content);
                errorDiv.classList.remove('active');
                editor.classList.remove('error');
                return true;
            } catch (e) {
                errorDiv.textContent = 'YAML Error: ' + e.message;
                errorDiv.classList.add('active');
                editor.classList.add('error');
                return false;
            }
        }

        async function savePrompt() {
            if (!validateYAML()) {
                showNotification('Please fix YAML errors before saving', true);
                return;
            }

            const content = document.getElementById('promptEditor').value;
            let topic = editingPromptTopic;
            
            if (isCreatingNew) {
                topic = document.getElementById('newPromptName').value.trim();
                if (!topic) {
                    showNotification('Please enter a topic name', true);
                    return;
                }
                
                topic = topic.toLowerCase().replace(/[^a-z0-9_]/g, '_');
            }
            
            try {
                const endpoint = isCreatingNew ? '/api/prompt/create' : '/api/prompt/save';
                const response = await fetch(endpoint, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        topic: topic,
                        content: content
                    })
                });
                
                const data = await response.json();
                
                if (data.success) {
                    showNotification(data.message);
                    closePromptEditor();
                    
                    if (isCreatingNew) {
                        await loadTopics();
                        await loadPrompts();
                    } else if (data.message.includes('reset')) {
                        document.getElementById('chatMessages').innerHTML = '';
                        await createSession();
                    }
                } else {
                    showNotification(data.message || 'Failed to save prompt', true);
                }
            } catch (error) {
                console.error('Error saving prompt:', error);
                showNotification('Failed to save prompt', true);
            }
        }

        async function deletePrompt(topic) {
            if (!confirm('Are you sure you want to delete "' + topic + '_prompt.yaml"?\n\nThis action cannot be undone.')) {
                return;
            }
            
            try {
                const response = await fetch('/api/prompt/delete', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        topic: topic
                    })
                });
                
                const data = await response.json();
                
                if (data.success) {
                    showNotification(data.message);
                    await loadTopics();
                    await loadPrompts();
                    
                    if (currentTopic === topic) {
                        const topicsResponse = await fetch('/api/topics');
                        const topicsData = await topicsResponse.json();
                        if (topicsData.success && topicsData.topics && topicsData.topics.length > 0) {
                            currentTopic = topicsData.topics[0];
                            document.getElementById('topicSelect').value = currentTopic;
                            await createSession();
                        } else {
                            document.getElementById('chatMessages').innerHTML = '';
                            document.getElementById('chatTitle').textContent = 'No prompts available';
                            document.getElementById('chatInfo').textContent = 'Create a new prompt to start chatting';
                            document.getElementById('sendBtn').disabled = true;
                            document.getElementById('assessmentBtn').disabled = true;
                            sessionActive = false;
                        }
                    }
                } else {
                    showNotification(data.message || 'Failed to delete prompt', true);
                }
            } catch (error) {
                console.error('Error deleting prompt:', error);
                showNotification('Failed to delete prompt', true);
            }
        }

        function showNotification(message, isError = false) {
            const notification = document.getElementById('notification');
            notification.textContent = message;
            notification.className = 'notification active' + (isError ? ' error' : '');
            setTimeout(() => {
                notification.classList.remove('active');
            }, 3000);
        }

        document.getElementById('topicSelect').addEventListener('change', async (e) => {
            currentTopic = e.target.value;
            await createSession();
        });

        document.querySelectorAll('.level-option').forEach(option => {
            option.addEventListener('click', async () => {
                document.querySelectorAll('.level-option').forEach(o => o.classList.remove('selected'));
                option.classList.add('selected');
                currentLevel = option.getAttribute('data-level');
                await createSession();
            });
        });

        async function createSession() {
            if (!currentTopic || !currentLevel) return;
            
            try {
                const response = await fetch('/api/create-session', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        topic: currentTopic,
                        level: currentLevel,
                        session_id: currentSessionID
                    })
                });
                
                const data = await response.json();
                
                if (data.success) {
                    sessionActive = true;
                    currentSessionID = data.session_id;
                    document.getElementById('chatTitle').textContent = data.topic + ' - ' + capitalizeLevel(data.level);
                    document.getElementById('chatInfo').textContent = 'Level: ' + capitalizeLevel(data.level);
                    document.getElementById('sendBtn').disabled = false;
                    document.getElementById('hintBtn').disabled = false;
                    document.getElementById('assessmentBtn').disabled = false;
                    
                    document.getElementById('chatMessages').innerHTML = '';
                    addMessage('assistant', data.message, null);
                }
            } catch (error) {
                console.error('Error creating session:', error);
            }
        }

        function capitalizeLevel(level) {
            return level.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
        }

        let isSending = false;

        document.getElementById('sendBtn').addEventListener('click', () => {
            if (!isSending) {
                sendMessage();
            }
        });
        
        document.getElementById('hintBtn').addEventListener('click', () => {
            showHint();
        });
        
        document.getElementById('assessmentBtn').addEventListener('click', () => {
            showAssessment();
        });
        
        document.getElementById('chatInput').addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey && !isSending) {
                e.preventDefault();
                sendMessage();
            }
        });

        async function sendMessage() {
            const input = document.getElementById('chatInput');
            const message = input.value.trim();
            input.value = '';
            
            if (!message || !sessionActive || isSending) return;
            
            isSending = true;
            addMessage('user', message, null);
            
            const sendBtn = document.getElementById('sendBtn');
            sendBtn.disabled = true;
            sendBtn.textContent = 'Sending...';
            
            const typingIndicator = addTypingIndicator();
            
            try {
                const eventSource = new EventSource('/api/stream?message=' + encodeURIComponent(message) + '&session_id=' + encodeURIComponent(currentSessionID));
                let messageStarted = false;
                let contentDiv, translationDiv;
                
                let userMessageDiv = null;
                const messagesContainer = document.getElementById('chatMessages');
                const userMessages = messagesContainer.querySelectorAll('.message.user');
                if (userMessages.length > 0) {
                    userMessageDiv = userMessages[userMessages.length - 1];
                }

                eventSource.onmessage = async (event) => {
                    const data = JSON.parse(event.data);
                    console.log('SSE Event received:', data.type, data);
                    
                    if (data.done) {
                        eventSource.close();
                        sendBtn.disabled = false;
                        sendBtn.textContent = 'Send';
                        isSending = false;
                        document.getElementById('chatInput').focus();
                        
                        if (translationDiv && contentDiv && contentDiv.textContent) {
                            translateMessage(contentDiv.textContent, translationDiv);
                        }
                    } else if (data.type === 'evaluation') {
                        console.log('Evaluation received:', data.data);
                        console.log('User message div:', userMessageDiv);
                        const evaluationDiv = document.createElement('div');
                        evaluationDiv.className = 'message-evaluation';
                        const statusEmoji = {
                            'excellent': '✨',
                            'good': '👍',
                            'needs_improvement': '📚'
                        };
                        const emoji = statusEmoji[data.data.status] || '✍️';
                        const statusText = data.data.status.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
                        evaluationDiv.innerHTML = '<div class="evaluation-header">' + emoji + ' ' + statusText + '</div><div class="evaluation-content">' +
                                '<div style="margin-bottom: 8px;"><b>' + data.data.short_description + '</b></div>' +
                                data.data.long_description +
                                (data.data.correct ? '<div style="margin-top: 8px; color: #2e7d32;"><b>✅ Correct:</b> ' + data.data.correct + '</div>' : '') +
                            '</div>';
                        if (userMessageDiv) {
                            console.log('Appending evaluation to user message');
                            userMessageDiv.appendChild(evaluationDiv);
                        } else {
                            console.error('userMessageDiv not found!');
                        }
                        scrollToBottom();
                    } else if (data.content) {
                        if (!messageStarted) {
                            removeTypingIndicator(typingIndicator);
                            const result = addMessage('assistant', '', null);
                            contentDiv = result.contentDiv;
                            translationDiv = result.translationDiv;
                            messageStarted = true;
                        }
                        contentDiv.textContent += data.content;
                        scrollToBottom();
                    }
                };
                
                eventSource.onerror = () => {
                    eventSource.close();
                    removeTypingIndicator(typingIndicator);
                    sendBtn.disabled = false;
                    sendBtn.textContent = 'Send';
                    isSending = false;
                };
            } catch (error) {
                console.error('Error sending message:', error);
                removeTypingIndicator(typingIndicator);
                sendBtn.disabled = false;
                sendBtn.textContent = 'Send';
                isSending = false;
            }
        }

        function addTypingIndicator() {
            const messagesDiv = document.getElementById('chatMessages');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message assistant';
            messageDiv.id = 'typing-indicator';
            
            const indicator = document.createElement('div');
            indicator.className = 'typing-indicator';
            indicator.innerHTML = '<span></span><span></span><span></span>';
            
            messageDiv.appendChild(indicator);
            messagesDiv.appendChild(messageDiv);
            scrollToBottom();
            
            return messageDiv;
        }

        function removeTypingIndicator(indicator) {
            if (indicator && indicator.parentNode) {
                indicator.parentNode.removeChild(indicator);
            }
        }

        async function translateMessage(text, translationDiv) {
            translationDiv.textContent = '🔄 Đang dịch...';
            translationDiv.classList.add('translation-loading');
            
            try {
                const response = await fetch('/api/translate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ text: text })
                });
                
                const data = await response.json();
                
                if (data.success && data.content) {
                    translationDiv.textContent = '🇻🇳 ' + data.content;
                    translationDiv.classList.remove('translation-loading');
                } else {
                    translationDiv.textContent = '';
                }
            } catch (error) {
                console.error('Translation error:', error);
                translationDiv.textContent = '';
            }
            
            scrollToBottom();
        }

        function addMessage(role, content, translation) {
            const messagesDiv = document.getElementById('chatMessages');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + role;
            
            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';
            contentDiv.textContent = content;
            
            messageDiv.appendChild(contentDiv);
            
            let translationDiv = null;
            if (role === 'assistant') {
                translationDiv = document.createElement('div');
                translationDiv.className = 'message-translation';
                messageDiv.appendChild(translationDiv);
                
                if (content) {
                    translateMessage(content, translationDiv);
                }
            }
            
            messagesDiv.appendChild(messageDiv);
            
            scrollToBottom();
            
            return { contentDiv, translationDiv };
        }

        function scrollToBottom() {
            const messagesDiv = document.getElementById('chatMessages');
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
        }

        function useSuggestion(text) {
            const input = document.getElementById('chatInput');
            const cleanText = text.replace(/[\u{1F300}-\u{1F9FF}]|[\u{2600}-\u{26FF}]|[\u{2700}-\u{27BF}]/gu, '').trim();
            input.value = cleanText;
            input.focus();
        }

        async function showHint() {
            if (!sessionActive) return;

            const messagesDiv = document.getElementById('chatMessages');
            const assistantMessages = messagesDiv.querySelectorAll('.message.assistant');
            if (assistantMessages.length === 0) return;
            
            const lastAssistantMessage = assistantMessages[assistantMessages.length - 1];
            const messageContent = lastAssistantMessage.querySelector('.message-content');
            if (!messageContent || !messageContent.textContent) return;
            
            const message = messageContent.textContent;
            
            const hintBtn = document.getElementById('hintBtn');
            const originalText = hintBtn.textContent;
            hintBtn.disabled = true;
            hintBtn.textContent = '⏳ Loading...';

            try {
                const response = await fetch('/api/suggestions', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ 
                        message: message,
                        session_id: currentSessionID
                    })
                });
                const data = await response.json();

                if (data.success && data.suggestions) {
                    const existingSuggestions = lastAssistantMessage.querySelector('.message-suggestions');
                    
                    if (existingSuggestions) {
                        existingSuggestions.remove();
                    }

                    const suggestionsDiv = document.createElement('div');
                    suggestionsDiv.className = 'message-suggestions';
                    const options = (data.suggestions.vocab_options || []).map(opt => 
                        '<div class="suggestion-option" onclick="useSuggestion(this.textContent)">' +
                        opt.emoji + ' ' + opt.text +
                        '</div>'
                    ).join('');
                    suggestionsDiv.innerHTML = '<div class="suggestions-header">💡 Suggested Responses</div><div class="suggestions-content">' +
                            (data.suggestions.leading_sentence ? '<div class="suggestion-lead">' + data.suggestions.leading_sentence + '</div>' : '') +
                            '<div class="suggestion-options">' + options + '</div></div>';
                    
                    lastAssistantMessage.appendChild(suggestionsDiv);
                    scrollToBottom();
                } else {
                    showNotification(data.message || 'Failed to get hints', true);
                }
            } catch (error) {
                console.error('Error getting hints:', error);
                showNotification('Failed to get hints', true);
            } finally {
                hintBtn.disabled = false;
                hintBtn.textContent = originalText;
            }
        }

        async function showAssessment() {
            if (!sessionActive) return;
            
            document.getElementById('assessmentModal').classList.add('active');
            
            const assessmentBtn = document.getElementById('assessmentBtn');
            const originalText = assessmentBtn.textContent;
            assessmentBtn.disabled = true;
            assessmentBtn.textContent = '⏳ Generating...';

            // Show initial loading state
            document.getElementById('assessmentContent').innerHTML = 
                '<div style="text-align: center; padding: 40px;">' +
                '<div style="font-size: 48px; margin-bottom: 20px;">⏳</div>' +
                '<div>Starting assessment...</div>' +
                '<div id="progressIndicator" style="margin-top: 20px; font-size: 14px; color: #666;"></div>' +
                '</div>';

            try {
                const eventSource = new EventSource('/api/assessment?session_id=' + encodeURIComponent(currentSessionID));
                
                eventSource.onmessage = (event) => {
                    const data = JSON.parse(event.data);
                    console.log('Assessment SSE Event:', data.type, data);
                    
                    if (data.done) {
                        eventSource.close();
                        assessmentBtn.disabled = false;
                        assessmentBtn.textContent = originalText;
                        
                        if (data.type === 'error') {
                            document.getElementById('assessmentContent').innerHTML = 
                                '<div style="text-align: center; padding: 40px; color: #f44336;">' +
                                '<div style="font-size: 48px; margin-bottom: 20px;">❌</div>' +
                                '<div>' + escapeHtml(data.error) + '</div>' +
                                '</div>';
                        } else if (data.type === 'assessment') {
                            displayAssessment(data.assessment);
                        }
                    } else if (data.type === 'progress') {
                        // Update progress indicator
                        const progressDiv = document.getElementById('progressIndicator');
                        if (progressDiv) {
                            const emoji = {
                                'level_assessment': '🔍',
                                'skills_evaluation': '📝',
                                'grammar_tips': '📚',
                                'vocabulary_tips': '📖',
                                'fluency_suggestions': '💬',
                                'vocabulary_suggestions': '🎯',
                                'completed': '✅'
                            };
                            const emojiIcon = emoji[data.data.type] || '⏳';
                            progressDiv.innerHTML = emojiIcon + ' ' + escapeHtml(data.data.message) + ' (' + data.data.progress + '%)';
                        }
                    }
                };
                
                eventSource.onerror = () => {
                    eventSource.close();
                    assessmentBtn.disabled = false;
                    assessmentBtn.textContent = originalText;
                    document.getElementById('assessmentContent').innerHTML = 
                        '<div style="text-align: center; padding: 40px; color: #f44336;">' +
                        '<div style="font-size: 48px; margin-bottom: 20px;">❌</div>' +
                        '<div>Failed to generate assessment</div>' +
                        '</div>';
                };
            } catch (error) {
                console.error('Error getting assessment:', error);
                assessmentBtn.disabled = false;
                assessmentBtn.textContent = originalText;
                document.getElementById('assessmentContent').innerHTML = 
                    '<div style="text-align: center; padding: 40px; color: #f44336;">' +
                    '<div style="font-size: 48px; margin-bottom: 20px;">❌</div>' +
                    '<div>Failed to generate assessment</div>' +
                    '</div>';
            }
        }

        function displayAssessment(assessment) {
            const content = document.getElementById('assessmentContent');
            
            console.log('Assessment object:', assessment);
            
            let html = '<div class="assessment-level">Level: ' + escapeHtml(assessment.level) + '</div>';
            
            if (assessment.general_skills) {
                html += '<div class="assessment-section">' +
                       '<h3>🎯 General Skills</h3>' +
                       '<div class="assessment-tip">' + escapeHtml(assessment.general_skills) + '</div>' +
                       '</div>';
            }
            
            if (assessment.grammar_tips && assessment.grammar_tips.length > 0) {
                html += '<div class="assessment-section">' +
                       '<h3>📚 Grammar Tips</h3>';
                assessment.grammar_tips.forEach(tip => {
                    html += '<div class="assessment-tip">' + escapeHtml(tip) + '</div>';
                });
                html += '</div>';
            }
            
            if (assessment.vocabulary_tips && assessment.vocabulary_tips.length > 0) {
                html += '<div class="assessment-section">' +
                       '<h3>📖 Vocabulary Tips</h3>';
                assessment.vocabulary_tips.forEach(tip => {
                    html += '<div class="assessment-tip">' + escapeHtml(tip) + '</div>';
                });
                html += '</div>';
            }
            
            if (assessment.fluency_suggestions && assessment.fluency_suggestions.length > 0) {
                html += '<div class="assessment-section">' +
                       '<h3>🗣️ Fluency Suggestions</h3>';
                assessment.fluency_suggestions.forEach(suggestion => {
                    html += '<div class="assessment-tip">' + escapeHtml(suggestion) + '</div>';
                });
                html += '</div>';
            }
            
            if (assessment.vocabulary_suggestions && assessment.vocabulary_suggestions.length > 0) {
                html += '<div class="assessment-section">' +
                       '<h3>📚 Vocabulary Suggestions</h3>';
                assessment.vocabulary_suggestions.forEach(suggestion => {
                    html += '<div class="assessment-tip">' + escapeHtml(suggestion) + '</div>';
                });
                html += '</div>';
            }
            
            content.innerHTML = html;
        }

        function escapeHtml(text) {
            if (typeof text !== 'string') return text;
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function closeAssessmentModal() {
            document.getElementById('assessmentModal').classList.remove('active');
        }

        init();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
