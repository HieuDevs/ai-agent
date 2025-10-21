package gateway

import (
	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/client"
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
	conversationSessions map[string]*managers.ConversationManager
	personalizeManager   *managers.PersonalizeManager
	mu                   sync.Mutex
	apiKey               string
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

type Lesson struct {
	Index         int    `json:"index"`
	Title         string `json:"title"`
	Prompt        string `json:"prompt"`
	Type          string `json:"type"`
	CharacterName string `json:"character_name"`
	Description   string `json:"description"`
	IsLocked      bool   `json:"is_locked"`
	Turns         int    `json:"turns"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type Chapter struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Lessons     []Lesson `json:"lessons"`
	IsLocked    bool     `json:"is_locked"`
	Order       int      `json:"order"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type LessonsResponse struct {
	Success  bool      `json:"success"`
	Chapters []Chapter `json:"chapters,omitzero"`
	Message  string    `json:"message,omitzero"`
}

func NewChatbotWeb(apiKey string) *ChatbotWeb {
	web := &ChatbotWeb{
		conversationSessions: make(map[string]*managers.ConversationManager),
		apiKey:               apiKey,
	}

	// Initialize PersonalizeManager once and reuse
	personalizeClient := client.NewOpenRouterClient(apiKey)
	web.personalizeManager = managers.NewPersonalizeManager(personalizeClient)

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
	// Personalize
	http.HandleFunc("/api/personalize", cw.handlePersonalize)
	// Prompts + Topics
	http.HandleFunc("/api/prompts", cw.handleGetPrompts)
	http.HandleFunc("/api/topics", cw.handleGetTopics)
	http.HandleFunc("/api/prompt/content", cw.handleGetPromptContent)
	http.HandleFunc("/api/prompt/save", cw.handleSavePrompt)
	http.HandleFunc("/api/prompt/create", cw.handleCreatePrompt)
	http.HandleFunc("/api/prompt/delete", cw.handleDeletePrompt)
	// Lessons
	http.HandleFunc("/api/lessons", cw.handleGetLessons)
	http.HandleFunc("/api/chapter/create", cw.handleCreateChapter)
	http.HandleFunc("/api/chapter/update", cw.handleUpdateChapter)
	http.HandleFunc("/api/chapter/delete", cw.handleDeleteChapter)
	http.HandleFunc("/api/lesson/create", cw.handleCreateLesson)
	http.HandleFunc("/api/lesson/update", cw.handleUpdateLesson)

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

	manager, exists := cw.conversationSessions[sessionID]
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
			// if suggestionAgent, ok := manager.GetAgent("SuggestionAgent"); ok {
			// 	suggestionJob := models.JobRequest{Task: "suggestion", LastAIMessage: aiResponse}
			// 	suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
			// 	if suggestionResponse.Success {
			// 		var suggestion models.SuggestionResponse
			// 		if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestion); err == nil {
			// 			historyManager.UpdateLastSuggestion(&suggestion)
			// 		}
			// 	}
			// }

			// Send message completion signal
			messageDoneData := map[string]any{
				"done": true,
				"type": "message",
			}
			messageDoneJSON, _ := json.Marshal(messageDoneData)
			fmt.Fprintf(w, "data: %s\n\n", messageDoneJSON)
			flusher.Flush()

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

			// Send evaluation completion signal
			evaluationDoneData := map[string]any{
				"done": true,
				"type": "evaluation",
			}
			evaluationDoneJSON, _ := json.Marshal(evaluationDoneData)
			fmt.Fprintf(w, "data: %s\n\n", evaluationDoneJSON)
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

// handlePersonalize generates a personalized lesson detail using the PersonalizeManager
func (cw *ChatbotWeb) handlePersonalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Topic    string `json:"topic"`
		Level    string `json:"level"`
		Language string `json:"language"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Topic == "" || req.Level == "" || req.Language == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Topic, level, and language are required",
		})
		return
	}

	task := models.JobRequest{
		Task: "create personalized lesson detail",
		Metadata: map[string]any{
			"topic":    req.Topic,
			"level":    req.Level,
			"language": req.Language,
		},
	}

	resp := cw.personalizeManager.ProcessTask(task)
	if !resp.Success {
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: resp.Error,
		})
		return
	}

	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Content: resp.Result,
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
		delete(cw.conversationSessions, sessionID)
	} else {
		sessionID = fmt.Sprintf("web_%d", utils.GetCurrentTimestamp())
	}

	manager := managers.NewConversationManager(cw.apiKey, level, req.Topic, userLanguage, sessionID)
	cw.conversationSessions[sessionID] = manager
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
	for _, manager := range cw.conversationSessions {
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

	manager, exists := cw.conversationSessions[req.SessionID]
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

func (cw *ChatbotWeb) handleGetLessons(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Read data from data.json file
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	// Parse JSON data
	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Return the parsed data
	json.NewEncoder(w).Encode(response)
}

func (cw *ChatbotWeb) handleCreateChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Order       int    `json:"order"`
		IsLocked    bool   `json:"is_locked"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Title == "" {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Title is required",
		})
		return
	}

	// Read current data
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Create new chapter
	newChapter := Chapter{
		ID:          fmt.Sprintf("chapter_%d", len(response.Chapters)+1),
		Title:       req.Title,
		Description: req.Description,
		Lessons:     []Lesson{},
		IsLocked:    req.IsLocked,
		Order:       req.Order,
		CreatedAt:   utils.GetCurrentTimestampString(),
		UpdatedAt:   utils.GetCurrentTimestampString(),
	}

	response.Chapters = append(response.Chapters, newChapter)

	// Save updated data
	updatedData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to serialize data: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile("data.json", updatedData, 0644); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to save data file: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(LessonsResponse{
		Success: true,
		Message: "Chapter created successfully",
	})
}

func (cw *ChatbotWeb) handleUpdateChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ChapterID   string `json:"chapter_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Order       int    `json:"order"`
		IsLocked    bool   `json:"is_locked"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.ChapterID == "" || req.Title == "" {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter ID and title are required",
		})
		return
	}

	// Read current data
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Find and update the chapter
	found := false
	for i := range response.Chapters {
		if response.Chapters[i].ID == req.ChapterID {
			response.Chapters[i].Title = req.Title
			response.Chapters[i].Description = req.Description
			response.Chapters[i].Order = req.Order
			response.Chapters[i].IsLocked = req.IsLocked
			response.Chapters[i].UpdatedAt = utils.GetCurrentTimestampString()
			found = true
			break
		}
	}

	if !found {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter not found",
		})
		return
	}

	// Save updated data
	updatedData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to serialize data: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile("data.json", updatedData, 0644); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to save data file: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(LessonsResponse{
		Success: true,
		Message: "Chapter updated successfully",
	})
}

func (cw *ChatbotWeb) handleDeleteChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ChapterID string `json:"chapter_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.ChapterID == "" {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter ID is required",
		})
		return
	}

	// Read current data
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Find and remove the chapter
	var updatedChapters []Chapter
	found := false
	for _, chapter := range response.Chapters {
		if chapter.ID != req.ChapterID {
			updatedChapters = append(updatedChapters, chapter)
		} else {
			found = true
		}
	}

	if !found {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter not found",
		})
		return
	}

	response.Chapters = updatedChapters

	// Save updated data
	updatedData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to serialize data: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile("data.json", updatedData, 0644); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to save data file: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(LessonsResponse{
		Success: true,
		Message: "Chapter deleted successfully",
	})
}

func (cw *ChatbotWeb) handleCreateLesson(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ChapterID     string `json:"chapter_id"`
		Title         string `json:"title"`
		CharacterName string `json:"character_name"`
		Prompt        string `json:"prompt"`
		Description   string `json:"description"`
		Turns         int    `json:"turns"`
		Type          string `json:"type"`
		IsLocked      bool   `json:"is_locked"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Title == "" || req.CharacterName == "" || req.Prompt == "" {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Title, character name, and prompt are required",
		})
		return
	}

	// Read current data
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Find the chapter
	var targetChapter *Chapter
	for i := range response.Chapters {
		if response.Chapters[i].ID == req.ChapterID {
			targetChapter = &response.Chapters[i]
			break
		}
	}

	if targetChapter == nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter not found",
		})
		return
	}

	// Create new lesson
	newLesson := Lesson{
		Index:         len(targetChapter.Lessons),
		Title:         req.Title,
		Prompt:        req.Prompt,
		Type:          req.Type,
		CharacterName: req.CharacterName,
		Description:   req.Description,
		IsLocked:      req.IsLocked,
		Turns:         req.Turns,
		CreatedAt:     utils.GetCurrentTimestampString(),
		UpdatedAt:     utils.GetCurrentTimestampString(),
	}

	targetChapter.Lessons = append(targetChapter.Lessons, newLesson)

	// Save updated data
	updatedData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to serialize data: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile("data.json", updatedData, 0644); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to save data file: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(LessonsResponse{
		Success: true,
		Message: "Lesson created successfully",
	})
}

func (cw *ChatbotWeb) handleUpdateLesson(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ChapterID     string `json:"chapter_id"`
		LessonIndex   int    `json:"lesson_index"`
		Title         string `json:"title"`
		CharacterName string `json:"character_name"`
		Prompt        string `json:"prompt"`
		Description   string `json:"description"`
		Turns         int    `json:"turns"`
		Type          string `json:"type"`
		IsLocked      bool   `json:"is_locked"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	if req.Title == "" || req.CharacterName == "" || req.Prompt == "" {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Title, character name, and prompt are required",
		})
		return
	}

	// Read current data
	data, err := os.ReadFile("data.json")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to read data file: " + err.Error(),
		})
		return
	}

	var response LessonsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to parse data file: " + err.Error(),
		})
		return
	}

	// Find the chapter and lesson
	var targetChapter *Chapter
	var targetLesson *Lesson
	for i := range response.Chapters {
		if response.Chapters[i].ID == req.ChapterID {
			targetChapter = &response.Chapters[i]
			for j := range targetChapter.Lessons {
				if targetChapter.Lessons[j].Index == req.LessonIndex {
					targetLesson = &targetChapter.Lessons[j]
					break
				}
			}
			break
		}
	}

	if targetChapter == nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Chapter not found",
		})
		return
	}

	if targetLesson == nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Lesson not found",
		})
		return
	}

	// Update lesson
	targetLesson.Title = req.Title
	targetLesson.CharacterName = req.CharacterName
	targetLesson.Prompt = req.Prompt
	targetLesson.Description = req.Description
	targetLesson.Turns = req.Turns
	targetLesson.Type = req.Type
	targetLesson.IsLocked = req.IsLocked
	targetLesson.UpdatedAt = utils.GetCurrentTimestampString()

	// Save updated data
	updatedData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to serialize data: " + err.Error(),
		})
		return
	}

	if err := os.WriteFile("data.json", updatedData, 0644); err != nil {
		json.NewEncoder(w).Encode(LessonsResponse{
			Success: false,
			Message: "Failed to save data file: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(LessonsResponse{
		Success: true,
		Message: "Lesson updated successfully",
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

	manager, exists := cw.conversationSessions[sessionID]
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
            max-height: 420px;
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
        
        .nav-actions {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .nav-tabs {
            display: flex;
            background: #f8f9fa;
            border-radius: 10px;
            padding: 4px;
            gap: 2px;
        }
        
        .nav-tab {
            padding: 10px 20px;
            background: transparent;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            font-size: 14px;
            color: #666;
            transition: all 0.2s;
        }
        
        .nav-tab.active {
            background: white;
            color: #333;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        
        .nav-tab:hover:not(.active) {
            background: rgba(255,255,255,0.5);
            color: #333;
        }
        
        .tab-content {
            display: none;
        }
        
        .tab-content.active {
            display: flex;
            flex-direction: column;
            flex: 1;
            min-height: 0; /* allow inner flex child to scroll */
        }
        
        .btn-nav {
            padding: 10px 16px;
            background: #ffffff;
            border: 2px solid #e0e0e0;
            color: #333;
            border-radius: 10px;
            cursor: pointer;
            font-weight: 600;
            font-size: 14px;
            transition: all 0.2s;
        }
        
        .btn-nav:hover {
            border-color: #667eea;
            color: #667eea;
            background: #f8f9ff;
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
            position: relative;
        }
        
        .audio-button {
            background: #667eea;
            color: white;
            border: none;
            border-radius: 6px;
            padding: 6px 12px;
            cursor: pointer;
            font-size: 12px;
            display: inline-flex;
            align-items: center;
            gap: 4px;
            transition: all 0.2s;
            opacity: 0.8;
            margin-top: 8px;
        }
        
        .audio-button:hover {
            opacity: 1;
            transform: scale(1.1);
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
        
        .btn-outline {
            padding: 10px 20px;
            background: white;
            color: #333;
            border: 2px solid #e0e0e0;
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
        
        .chapter-card {
            background: white;
            border-radius: 12px;
            border: 1px solid #e0e0e0;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }

        .chapter-header {
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .chapter-title {
            font-size: 18px;
            font-weight: 600;
            margin: 0;
        }

        .chapter-description {
            font-size: 14px;
            opacity: 0.9;
            margin: 5px 0 0 0;
        }

        .chapter-actions {
            display: flex;
            gap: 10px;
        }

        .btn-chapter-action {
            padding: 8px 16px;
            background: rgba(255,255,255,0.2);
            color: white;
            border: 1px solid rgba(255,255,255,0.3);
            border-radius: 6px;
            cursor: pointer;
            font-size: 12px;
            font-weight: 600;
            transition: all 0.2s;
        }

        .btn-chapter-action:hover {
            background: rgba(255,255,255,0.3);
            transform: translateY(-1px);
        }

        .lessons-list {
            padding: 20px;
        }

        .lesson-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 15px;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            margin-bottom: 10px;
            background: #f9fafb;
            transition: all 0.2s;
        }

        .lesson-item:hover {
            background: #f0f0f0;
            border-color: #667eea;
        }

        .lesson-info {
            flex: 1;
        }

        .lesson-title {
            font-weight: 600;
            color: #333;
            margin-bottom: 5px;
        }

        .lesson-details {
            font-size: 13px;
            color: #666;
            display: flex;
            gap: 15px;
        }

        .lesson-status {
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .status-badge {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 600;
        }

        .status-completed {
            background: #e8f5e9;
            color: #2e7d32;
        }

        .status-locked {
            background: #ffebee;
            color: #c62828;
        }

        .status-available {
            background: #e3f2fd;
            color: #1565c0;
        }

        .lesson-actions {
            display: flex;
            gap: 8px;
        }

        .btn-lesson-action {
            padding: 6px 12px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 11px;
            font-weight: 600;
            transition: all 0.2s;
        }

        .btn-lesson-edit {
            background: #667eea;
            color: white;
        }

        .btn-lesson-edit:hover {
            background: #5568d3;
            transform: translateY(-1px);
        }

        .btn-lesson-delete {
            background: #f44336;
            color: white;
        }

        .btn-lesson-delete:hover {
            background: #d32f2f;
            transform: translateY(-1px);
        }

        .btn-lesson-add {
            background: #4CAF50;
            color: white;
        }

        .btn-lesson-add:hover {
            background: #45a049;
            transform: translateY(-1px);
        }


        @media (max-width: 768px) {
            .sidebar {
                width: 280px;
            }
            
            .level-grid {
                grid-template-columns: 1fr;
            }

            .chapter-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 10px;
            }

            .lesson-item {
                flex-direction: column;
                align-items: flex-start;
                gap: 10px;
            }

            .lesson-details {
                flex-direction: column;
                gap: 5px;
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
            <div class="nav-actions">
                <div class="nav-tabs">
                    <button id="conversationTab" class="nav-tab active" onclick="switchTab('conversation')">💬 Conversation</button>
                    <button id="personalizeTab" class="nav-tab" onclick="switchTab('personalize')">✨ Personalize</button>
                    <button id="lessonsTab" class="nav-tab" onclick="switchTab('lessons')">📚 Lessons</button>
                </div>
            </div>
        </div>
        <div id="conversationContent" class="tab-content active">
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
        
        <div id="personalizeContent" class="tab-content">
            <div class="sidebar-content" style="padding: 20px;">
                <div class="section">
                    <div class="section-title">Topic</div>
                    <input id="personalizeTopic" class="input-topic-name" placeholder="Enter topic (e.g., travel, music)" />
                </div>
                <div class="section">
                    <div class="section-title">Level</div>
                    <select id="personalizeLevel" class="form-select">
                        <option value="beginner">Beginner</option>
                        <option value="elementary">Elementary</option>
                        <option value="intermediate" selected>Intermediate</option>
                        <option value="upper_intermediate">Upper Intermediate</option>
                        <option value="advanced">Advanced</option>
                        <option value="fluent">Fluent</option>
                    </select>
                </div>
                <div class="section">
                    <div class="section-title">Native Language</div>
                    <input id="personalizeLanguage" class="input-topic-name" placeholder="Enter native language (e.g., Vietnamese)" />
                </div>
                <div id="personalizeError" class="yaml-error"></div>
                <div id="personalizeLoading" class="translation-loading" style="display:none; margin-top: 10px;">⏳ Generating personalized lesson...</div>
                <div class="section">
                    <div class="section-title">Result</div>
                    <div id="personalizeResult" style="background:#f9fafb; padding:12px; border-radius:8px; overflow:auto; max-height:50vh; font-size:14px; line-height:1.6;"></div>
                </div>
                <div style="margin-top: 20px;">
                    <button id="personalizeGenerateBtn" class="btn-primary" onclick="submitPersonalize()" style="width: 100%;">Generate Personalized Lesson</button>
                </div>
            </div>
        </div>
        
        <div id="lessonsContent" class="tab-content">
            <div style="padding: 20px; height: 100%; overflow-y: auto;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
                    <h2 style="margin: 0; color: #333;">📚 Lesson Management</h2>
                    <button class="btn-primary" onclick="openNewChapterDialog()">+ Add Chapter</button>
                </div>
                
                <div id="lessonsContainer" style="display: flex; flex-direction: column; gap: 20px;">
                    <div style="text-align: center; padding: 40px; color: #666;">
                        <div style="font-size: 48px; margin-bottom: 20px;">⏳</div>
                        <div>Loading lessons...</div>
                    </div>
                </div>
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
    
    <div id="chapterModal" class="modal">
        <div class="modal-content" style="max-width: 1000px;">
            <div class="modal-header">
                <div class="modal-title" id="chapterModalTitle">Add New Chapter</div>
                <button class="btn-close" onclick="closeChapterModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div style="display: flex; gap: 20px;">
                    <div style="flex: 1;">
                        <div class="section">
                            <div class="section-title">Chapter Title</div>
                            <input id="chapterTitle" class="input-topic-name" placeholder="Enter chapter title" />
                        </div>
                        <div class="section">
                            <div class="section-title">Description</div>
                            <textarea id="chapterDescription" class="input-topic-name" placeholder="Enter chapter description" rows="3"></textarea>
                        </div>
                        <div class="section">
                            <div class="section-title">Order</div>
                            <input id="chapterOrder" class="input-topic-name" type="number" placeholder="Enter order number" />
                        </div>
                        <div class="section">
                            <div class="section-title">Locked</div>
                            <select id="chapterLocked" class="form-select">
                                <option value="false">Unlocked</option>
                                <option value="true">Locked</option>
                            </select>
                        </div>
                    </div>
                    <div style="flex: 1;">
                        <div class="section">
                            <div class="section-title">Lessons in this Chapter</div>
                            <div id="chapterLessonsList" style="max-height: 300px; overflow-y: auto; border: 1px solid #e0e0e0; border-radius: 8px; padding: 10px;">
                                <div style="text-align: center; color: #666; padding: 20px;">
                                    No lessons yet
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn-secondary" onclick="closeChapterModal()">Cancel</button>
                <button id="chapterDeleteBtn" class="btn-outline" onclick="deleteChapterFromModal()" style="display: none; background: #f44336; color: white; border-color: #f44336;">Delete Chapter</button>
                <button class="btn-primary" onclick="saveChapter()">Save Chapter</button>
            </div>
        </div>
    </div>
    
    <div id="lessonModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <div class="modal-title" id="lessonModalTitle">Add New Lesson</div>
                <button class="btn-close" onclick="closeLessonModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="section">
                    <div class="section-title">Lesson Title</div>
                    <input id="lessonTitle" class="input-topic-name" placeholder="Enter lesson title" />
                </div>
                <div class="section">
                    <div class="section-title">Character Name</div>
                    <input id="lessonCharacter" class="input-topic-name" placeholder="Enter character name" />
                </div>
                <div class="section">
                    <div class="section-title">Prompt</div>
                    <input id="lessonPrompt" class="input-topic-name" placeholder="Enter prompt name" />
                </div>
                <div class="section">
                    <div class="section-title">Description</div>
                    <textarea id="lessonDescription" class="input-topic-name" placeholder="Enter lesson description" rows="3"></textarea>
                </div>
                <div class="section">
                    <div class="section-title">Turns</div>
                    <input id="lessonTurns" class="input-topic-name" type="number" placeholder="Enter number of turns" />
                </div>
                <div class="section">
                    <div class="section-title">Type</div>
                    <select id="lessonType" class="form-select">
                        <option value="Conversation">Conversation</option>
                        <option value="Exercise">Exercise</option>
                        <option value="Quiz">Quiz</option>
                    </select>
                </div>
                <div class="section">
                    <div class="section-title">Status</div>
                    <select id="lessonStatus" class="form-select">
                        <option value="available">Available</option>
                        <option value="locked">Locked</option>
                    </select>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn-secondary" onclick="closeLessonModal()">Cancel</button>
                <button class="btn-primary" onclick="saveLesson()">Save Lesson</button>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.min.js"></script>
    <script>
        let currentTopic = '';
        let currentLevel = 'intermediate';
        let sessionActive = false;
        let editingPromptTopic = '';
        let isCreatingNew = false;
        let yamlValidationTimeout = null;
        let currentSessionID = '';
        let currentChapterId = '';
        let editingChapterId = '';
        let editingLessonIndex = -1;

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

        function switchTab(tabName) {
            // Update tab buttons
            document.querySelectorAll('.nav-tab').forEach(tab => tab.classList.remove('active'));
            document.getElementById(tabName + 'Tab').classList.add('active');
            
            // Update tab content
            document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
            document.getElementById(tabName + 'Content').classList.add('active');
            
            // Reset personalize form when switching to personalize tab
            if (tabName === 'personalize') {
                document.getElementById('personalizeTopic').value = '';
                document.getElementById('personalizeLevel').value = currentLevel || 'intermediate';
                document.getElementById('personalizeLanguage').value = 'Vietnamese';
                document.getElementById('personalizeError').classList.remove('active');
                document.getElementById('personalizeError').textContent = '';
                document.getElementById('personalizeLoading').style.display = 'none';
                document.getElementById('personalizeResult').textContent = '';
                // Hide sidebar in personalize tab
                const sidebar = document.querySelector('.sidebar');
                if (sidebar) sidebar.style.display = 'none';
            } else if (tabName === 'conversation') {
                // Show sidebar in conversation tab
                const sidebar = document.querySelector('.sidebar');
                if (sidebar) sidebar.style.display = '';
                // Ensure messages area fills and scrolls, and focus input at bottom
                setTimeout(() => {
                    scrollToBottom();
                    document.getElementById('chatInput').focus();
                }, 0);
            } else if (tabName === 'lessons') {
                // Hide sidebar in lessons tab
                const sidebar = document.querySelector('.sidebar');
                if (sidebar) sidebar.style.display = 'none';
                // Load lessons data
                loadLessons();
            }
        }

        async function submitPersonalize() {
            const topic = document.getElementById('personalizeTopic').value.trim();
            const level = document.getElementById('personalizeLevel').value;
            const language = document.getElementById('personalizeLanguage').value.trim() || 'Vietnamese';
            const errorDiv = document.getElementById('personalizeError');
            const loadingDiv = document.getElementById('personalizeLoading');
            const resultDiv = document.getElementById('personalizeResult');
            const generateBtn = document.getElementById('personalizeGenerateBtn');

            if (!topic) {
                errorDiv.textContent = 'Please enter a topic';
                errorDiv.classList.add('active');
                return;
            }

            try {
                loadingDiv.style.display = 'block';
                errorDiv.classList.remove('active');
                errorDiv.textContent = '';
                resultDiv.textContent = '';
                if (generateBtn) { generateBtn.disabled = true; generateBtn.textContent = '⏳ Generating...'; }
                const response = await fetch('/api/personalize', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ topic, level, language })
                });
                const data = await response.json();
                if (data.success) {
                    // Pretty-print JSON result (handle optional code fences)
                    let raw = (data.content || '').trim();
                    const bt = String.fromCharCode(96); // backtick
                    const fence = bt + bt + bt;
                    if (raw.startsWith(fence + 'json')) raw = raw.slice(fence.length + 4);
                    if (raw.startsWith(fence)) raw = raw.slice(fence.length);
                    if (raw.endsWith(fence)) raw = raw.slice(0, -fence.length);
                    raw = raw.trim();
                    try {
                        const obj = JSON.parse(raw);
                        // Create formatted JSON with syntax highlighting
                        resultDiv.innerHTML = '<pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 8px; border: 1px solid #333; font-family: \'Courier New\', monospace; font-size: 13px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word;">' + 
                            formatJSON(obj) + '</pre>';
                    } catch (_) {
                        // Fallback to raw text if parse fails
                        resultDiv.innerHTML = '<pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 8px; border: 1px solid #333; font-family: \'Courier New\', monospace; font-size: 13px; line-height: 1.5; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word;">' + 
                            escapeHtml(raw || data.content) + '</pre>';
                    }
                } else {
                    errorDiv.textContent = data.message || 'Failed to generate personalized lesson';
                    errorDiv.classList.add('active');
                }
            } catch (e) {
                errorDiv.textContent = 'Network error: ' + e.message;
                errorDiv.classList.add('active');
            } finally {
                loadingDiv.style.display = 'none';
                if (generateBtn) { generateBtn.disabled = false; generateBtn.textContent = 'Generate Personalized Lesson'; }
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
            input.focus();
            
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
                    
                    if (data.done && data.type === 'message') {
                        // Message streaming is complete, trigger translation and Google Translate
                        if (translationDiv && contentDiv && contentDiv.textContent) {
                            translateMessage(contentDiv.textContent, translationDiv);
                            // Use Google Translate to read the English text
                            readWithGoogleTranslate(contentDiv.textContent);
                        }
                        // Add audio button to the completed message
                        addAudioButtonToLastMessage();
                    } else if (data.done && data.type === 'evaluation') {
                        // Stream is completely finished
                        eventSource.close();
                        sendBtn.disabled = false;
                        sendBtn.textContent = 'Send';
                        isSending = false;
                        document.getElementById('chatInput').focus();
                    } else if (data.type === 'evaluation' && !data.done) {
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
            
            // Add audio button below content for assistant messages
            if (role === 'assistant' && content) {
                const audioButton = document.createElement('button');
                audioButton.className = 'audio-button';
                audioButton.innerHTML = '🔊 Play Audio';
                audioButton.title = 'Play audio';
                audioButton.onclick = function() {
                    readWithGoogleTranslate(content);
                };
                messageDiv.appendChild(audioButton);
            }
            
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

        function addAudioButtonToLastMessage() {
            const messagesDiv = document.getElementById('chatMessages');
            const assistantMessages = messagesDiv.querySelectorAll('.message.assistant');
            if (assistantMessages.length === 0) return;
            
            const lastMessage = assistantMessages[assistantMessages.length - 1];
            const contentDiv = lastMessage.querySelector('.message-content');
            if (!contentDiv || !contentDiv.textContent) return;
            
            // Check if audio button already exists
            if (lastMessage.querySelector('.audio-button')) return;
            
            const content = contentDiv.textContent;
            const audioButton = document.createElement('button');
            audioButton.className = 'audio-button';
            audioButton.innerHTML = '🔊 Play Audio';
            audioButton.title = 'Play audio';
            audioButton.onclick = function() {
                readWithGoogleTranslate(content);
            };
            
            // Insert before translation div if it exists, otherwise just append
            const translationDiv = lastMessage.querySelector('.message-translation');
            if (translationDiv) {
                lastMessage.insertBefore(audioButton, translationDiv);
            } else {
                lastMessage.appendChild(audioButton);
            }
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

        function formatJSON(obj, indent = 0) {
            const spaces = '  '.repeat(indent);
            if (obj === null) return '<span style="color: #608b4e;">null</span>';
            if (typeof obj === 'string') return '<span style="color: #ce9178;">"' + escapeHtml(obj) + '"</span>';
            if (typeof obj === 'number') return '<span style="color: #b5cea8;">' + obj + '</span>';
            if (typeof obj === 'boolean') return '<span style="color: #569cd6;">' + obj + '</span>';
            
            if (Array.isArray(obj)) {
                if (obj.length === 0) return '[]';
                let result = '[\n';
                obj.forEach((item, index) => {
                    result += spaces + '  ' + formatJSON(item, indent + 1);
                    if (index < obj.length - 1) result += ',';
                    result += '\n';
                });
                result += spaces + ']';
                return result;
            }
            
            if (typeof obj === 'object') {
                const keys = Object.keys(obj);
                if (keys.length === 0) return '{}';
                let result = '{\n';
                keys.forEach((key, index) => {
                    result += spaces + '  <span style="color: #9cdcfe;">"' + escapeHtml(key) + '"</span>: ' + formatJSON(obj[key], indent + 1);
                    if (index < keys.length - 1) result += ',';
                    result += '\n';
                });
                result += spaces + '}';
                return result;
            }
            
            return escapeHtml(String(obj));
        }

        function closeAssessmentModal() {
            document.getElementById('assessmentModal').classList.remove('active');
        }

        function readWithGoogleTranslate(text) {
            if (!text || text.trim() === '') return;
            
            // Try Web Speech API first (more reliable)
            if ('speechSynthesis' in window) {
                const utterance = new SpeechSynthesisUtterance(text);
                utterance.lang = 'en-US';
                utterance.rate = 1;
                utterance.pitch = 1;
                speechSynthesis.speak(utterance);
                return;
            }
            
            // Fallback: Create audio element with Google TTS
            const audio = document.createElement('audio');
            audio.style.display = 'none';
            
            const encodedText = encodeURIComponent(text);
            const ttsUrl = "https://translate.google.com/translate_tts?ie=UTF-8&client=tw-ob&q=" + encodedText + "&tl=en";
            
            audio.src = ttsUrl;
            audio.autoplay = true;
            audio.onloadstart = function() {
                document.body.appendChild(audio);
            };
            audio.onended = function() {
                if (audio.parentNode) {
                    audio.parentNode.removeChild(audio);
                }
            };
            audio.onerror = function() {
                if (audio.parentNode) {
                    audio.parentNode.removeChild(audio);
                }
                console.log('Audio playback failed');
            };
            
            // Try to play
            audio.play().catch(function(error) {
                console.log('Audio play failed:', error);
                if (audio.parentNode) {
                    audio.parentNode.removeChild(audio);
                }
            });
        }

        async function loadLessons() {
            try {
                const response = await fetch('/api/lessons');
                const data = await response.json();
                
                if (data.success && data.chapters) {
                    displayLessons(data.chapters);
                } else {
                    document.getElementById('lessonsContainer').innerHTML = 
                        '<div style="text-align: center; padding: 40px; color: #f44336;">' +
                        '<div style="font-size: 48px; margin-bottom: 20px;">❌</div>' +
                        '<div>Failed to load lessons: ' + (data.message || 'Unknown error') + '</div>' +
                        '</div>';
                }
            } catch (error) {
                console.error('Error loading lessons:', error);
                document.getElementById('lessonsContainer').innerHTML = 
                    '<div style="text-align: center; padding: 40px; color: #f44336;">' +
                    '<div style="font-size: 48px; margin-bottom: 20px;">❌</div>' +
                    '<div>Failed to load lessons</div>' +
                    '</div>';
            }
        }

        function displayLessons(chapters) {
            const container = document.getElementById('lessonsContainer');
            container.innerHTML = '';
            
            chapters.forEach(chapter => {
                const chapterCard = document.createElement('div');
                chapterCard.className = 'chapter-card';
                chapterCard.innerHTML = 
                    '<div class="chapter-header">' +
                        '<div>' +
                            '<h3 class="chapter-title">' + escapeHtml(chapter.title) + '</h3>' +
                            '<p class="chapter-description">' + escapeHtml(chapter.description) + '</p>' +
                        '</div>' +
                        '<div class="chapter-actions">' +
                            '<button class="btn-chapter-action" onclick="editChapter(\'' + chapter.id + '\')">Edit</button>' +
                            '<button class="btn-chapter-action" onclick="addLesson(\'' + chapter.id + '\')">+ Lesson</button>' +
                            '<button class="btn-chapter-action" onclick="deleteChapter(\'' + chapter.id + '\')">Delete</button>' +
                        '</div>' +
                    '</div>' +
                    '<div class="lessons-list">' +
                        chapter.lessons.map(lesson => createLessonHTML(lesson, chapter.id)).join('') +
                    '</div>';
                container.appendChild(chapterCard);
            });
        }

        function createLessonHTML(lesson, chapterId) {
            let statusClass = 'status-available';
            let statusText = 'Available';
            
            if (lesson.is_locked) {
                statusClass = 'status-locked';
                statusText = 'Locked';
            }
            
            return 
                '<div class="lesson-item">' +
                    '<div class="lesson-info">' +
                        '<div class="lesson-title">' + escapeHtml(lesson.title) + '</div>' +
                        '<div class="lesson-details">' +
                            '<span>Character: ' + escapeHtml(lesson.character_name) + '</span>' +
                            '<span>Turns: ' + lesson.turns + '</span>' +
                            '<span>Type: ' + escapeHtml(lesson.type) + '</span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="lesson-status">' +
                        '<span class="status-badge ' + statusClass + '">' + statusText + '</span>' +
                        '<div class="lesson-actions">' +
                            '<button class="btn-lesson-action btn-lesson-edit" onclick="editLesson(\'' + chapterId + '\', ' + lesson.index + ')">Edit</button>' +
                            '<button class="btn-lesson-action btn-lesson-delete" onclick="deleteLesson(\'' + chapterId + '\', ' + lesson.index + ')">Delete</button>' +
                        '</div>' +
                    '</div>' +
                '</div>';
        }

        function openNewChapterDialog() {
            editingChapterId = '';
            document.getElementById('chapterModalTitle').textContent = 'Add New Chapter';
            document.getElementById('chapterTitle').value = '';
            document.getElementById('chapterDescription').value = '';
            document.getElementById('chapterOrder').value = '';
            document.getElementById('chapterLocked').value = 'false';
            document.getElementById('chapterDeleteBtn').style.display = 'none';
            document.getElementById('chapterLessonsList').innerHTML = '<div style="text-align: center; color: #666; padding: 20px;">No lessons yet</div>';
            document.getElementById('chapterModal').classList.add('active');
        }

        async function editChapter(chapterId) {
            editingChapterId = chapterId;
            document.getElementById('chapterModalTitle').textContent = 'Edit Chapter';
            document.getElementById('chapterDeleteBtn').style.display = 'inline-block';
            
            try {
                const response = await fetch('/api/lessons');
                const data = await response.json();
                
                if (data.success && data.chapters) {
                    const chapter = data.chapters.find(ch => ch.id === chapterId);
                    if (chapter) {
                        // Populate form fields
                        document.getElementById('chapterTitle').value = chapter.title;
                        document.getElementById('chapterDescription').value = chapter.description;
                        document.getElementById('chapterOrder').value = chapter.order;
                        document.getElementById('chapterLocked').value = chapter.is_locked.toString();
                        
                        // Display lessons
                        displayChapterLessons(chapter.lessons);
                    }
                }
            } catch (error) {
                console.error('Error loading chapter data:', error);
                showNotification('Failed to load chapter data', true);
            }
            
            document.getElementById('chapterModal').classList.add('active');
        }

        function displayChapterLessons(lessons) {
            const container = document.getElementById('chapterLessonsList');
            
            if (lessons.length === 0) {
                container.innerHTML = '<div style="text-align: center; color: #666; padding: 20px;">No lessons yet</div>';
                return;
            }
            
            container.innerHTML = lessons.map(lesson => {
                let statusClass = 'status-available';
                let statusText = 'Available';
                
                if (lesson.is_locked) {
                    statusClass = 'status-locked';
                    statusText = 'Locked';
                }
                
                return '<div style="display: flex; justify-content: space-between; align-items: center; padding: 10px; border: 1px solid #e0e0e0; border-radius: 6px; margin-bottom: 8px; background: #f9fafb;">' +
                    '<div>' +
                        '<div style="font-weight: 600; color: #333;">' + escapeHtml(lesson.title) + '</div>' +
                        '<div style="font-size: 12px; color: #666;">' + escapeHtml(lesson.character_name) + ' • ' + lesson.turns + ' turns</div>' +
                    '</div>' +
                    '<div style="display: flex; align-items: center; gap: 8px;">' +
                        '<span class="status-badge ' + statusClass + '">' + statusText + '</span>' +
                        '<button class="btn-lesson-action btn-lesson-edit" onclick="editLessonFromChapter(\'' + editingChapterId + '\', ' + lesson.index + ')">Edit</button>' +
                    '</div>' +
                '</div>';
            }).join('');
        }

        function addLesson(chapterId) {
            currentChapterId = chapterId;
            editingLessonIndex = -1;
            document.getElementById('lessonModalTitle').textContent = 'Add New Lesson';
            document.getElementById('lessonTitle').value = '';
            document.getElementById('lessonCharacter').value = '';
            document.getElementById('lessonPrompt').value = '';
            document.getElementById('lessonDescription').value = '';
            document.getElementById('lessonTurns').value = '9';
            document.getElementById('lessonType').value = 'Conversation';
            document.getElementById('lessonStatus').value = 'available';
            document.getElementById('lessonModal').classList.add('active');
        }

        async function editLesson(chapterId, lessonIndex) {
            currentChapterId = chapterId;
            editingLessonIndex = lessonIndex;
            document.getElementById('lessonModalTitle').textContent = 'Edit Lesson';
            
            try {
                const response = await fetch('/api/lessons');
                const data = await response.json();
                
                if (data.success && data.chapters) {
                    const chapter = data.chapters.find(ch => ch.id === chapterId);
                    if (chapter) {
                        const lesson = chapter.lessons.find(l => l.index === lessonIndex);
                        if (lesson) {
                            // Populate form fields
                            document.getElementById('lessonTitle').value = lesson.title;
                            document.getElementById('lessonCharacter').value = lesson.character_name;
                            document.getElementById('lessonPrompt').value = lesson.prompt;
                            document.getElementById('lessonDescription').value = lesson.description;
                            document.getElementById('lessonTurns').value = lesson.turns;
                            document.getElementById('lessonType').value = lesson.type;
                            document.getElementById('lessonStatus').value = lesson.is_locked ? 'locked' : 'available';
                        }
                    }
                }
            } catch (error) {
                console.error('Error loading lesson data:', error);
                showNotification('Failed to load lesson data', true);
            }
            
            document.getElementById('lessonModal').classList.add('active');
        }

        function closeChapterModal() {
            document.getElementById('chapterModal').classList.remove('active');
        }

        function closeLessonModal() {
            document.getElementById('lessonModal').classList.remove('active');
        }

        async function saveChapter() {
            const title = document.getElementById('chapterTitle').value.trim();
            const description = document.getElementById('chapterDescription').value.trim();
            const order = parseInt(document.getElementById('chapterOrder').value) || 1;
            const isLocked = document.getElementById('chapterLocked').value === 'true';

            if (!title) {
                showNotification('Please enter a chapter title', true);
                return;
            }

            try {
                const isEditing = editingChapterId !== '';
                const url = isEditing ? '/api/chapter/update' : '/api/chapter/create';
                const body = isEditing ? {
                    chapter_id: editingChapterId,
                    title: title,
                    description: description,
                    order: order,
                    is_locked: isLocked
                } : {
                    title: title,
                    description: description,
                    order: order,
                    is_locked: isLocked
                };

                const response = await fetch(url, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(body)
                });

                const data = await response.json();
                
                if (data.success) {
                    showNotification(isEditing ? 'Chapter updated successfully!' : 'Chapter created successfully!');
                    closeChapterModal();
                    loadLessons();
                } else {
                    showNotification(data.message || (isEditing ? 'Failed to update chapter' : 'Failed to create chapter'), true);
                }
            } catch (error) {
                console.error('Error saving chapter:', error);
                showNotification('Failed to save chapter', true);
            }
        }

        async function saveLesson() {
            const title = document.getElementById('lessonTitle').value.trim();
            const character = document.getElementById('lessonCharacter').value.trim();
            const prompt = document.getElementById('lessonPrompt').value.trim();
            const description = document.getElementById('lessonDescription').value.trim();
            const turns = parseInt(document.getElementById('lessonTurns').value) || 9;
            const type = document.getElementById('lessonType').value;
            const status = document.getElementById('lessonStatus').value;

            if (!title || !character || !prompt) {
                showNotification('Please fill in all required fields', true);
                return;
            }

            const isLocked = status === 'locked';

            try {
                const url = editingLessonIndex >= 0 ? '/api/lesson/update' : '/api/lesson/create';
                const body = editingLessonIndex >= 0 ? {
                    chapter_id: currentChapterId,
                    lesson_index: editingLessonIndex,
                    title: title,
                    character_name: character,
                    prompt: prompt,
                    description: description,
                    turns: turns,
                    type: type,
                    is_locked: isLocked
                } : {
                    chapter_id: currentChapterId,
                    title: title,
                    character_name: character,
                    prompt: prompt,
                    description: description,
                    turns: turns,
                    type: type,
                    is_locked: isLocked
                };

                const response = await fetch(url, {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(body)
                });

                const data = await response.json();
                
                if (data.success) {
                    showNotification(editingLessonIndex >= 0 ? 'Lesson updated successfully!' : 'Lesson created successfully!');
                    closeLessonModal();
                    loadLessons();
                } else {
                    showNotification(data.message || (editingLessonIndex >= 0 ? 'Failed to update lesson' : 'Failed to create lesson'), true);
                }
            } catch (error) {
                console.error('Error saving lesson:', error);
                showNotification(editingLessonIndex >= 0 ? 'Failed to update lesson' : 'Failed to create lesson', true);
            }
        }

        async function deleteChapter(chapterId) {
            if (!confirm('Are you sure you want to delete this chapter? This action cannot be undone.')) {
                return;
            }
            
            try {
                const response = await fetch('/api/chapter/delete', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        chapter_id: chapterId
                    })
                });

                const data = await response.json();
                
                if (data.success) {
                    showNotification('Chapter deleted successfully!');
                    loadLessons();
                } else {
                    showNotification(data.message || 'Failed to delete chapter', true);
                }
            } catch (error) {
                console.error('Error deleting chapter:', error);
                showNotification('Failed to delete chapter', true);
            }
        }

        async function deleteChapterFromModal() {
            if (!confirm('Are you sure you want to delete this chapter? This action cannot be undone.')) {
                return;
            }
            
            try {
                const response = await fetch('/api/chapter/delete', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        chapter_id: editingChapterId
                    })
                });

                const data = await response.json();
                
                if (data.success) {
                    showNotification('Chapter deleted successfully!');
                    closeChapterModal();
                    loadLessons();
                } else {
                    showNotification(data.message || 'Failed to delete chapter', true);
                }
            } catch (error) {
                console.error('Error deleting chapter:', error);
                showNotification('Failed to delete chapter', true);
            }
        }

        async function editLessonFromChapter(chapterId, lessonIndex) {
            currentChapterId = chapterId;
            editingLessonIndex = lessonIndex;
            document.getElementById('lessonModalTitle').textContent = 'Edit Lesson';
            
            try {
                const response = await fetch('/api/lessons');
                const data = await response.json();
                
                if (data.success && data.chapters) {
                    const chapter = data.chapters.find(ch => ch.id === chapterId);
                    if (chapter) {
                        const lesson = chapter.lessons.find(l => l.index === lessonIndex);
                        if (lesson) {
                            // Populate form fields
                            document.getElementById('lessonTitle').value = lesson.title;
                            document.getElementById('lessonCharacter').value = lesson.character_name;
                            document.getElementById('lessonPrompt').value = lesson.prompt;
                            document.getElementById('lessonDescription').value = lesson.description;
                            document.getElementById('lessonTurns').value = lesson.turns;
                            document.getElementById('lessonType').value = lesson.type;
                            document.getElementById('lessonStatus').value = lesson.is_locked ? 'locked' : 'available';
                        }
                    }
                }
            } catch (error) {
                console.error('Error loading lesson data:', error);
                showNotification('Failed to load lesson data', true);
            }
            
            document.getElementById('lessonModal').classList.add('active');
        }

        function deleteLesson(chapterId, lessonIndex) {
            if (!confirm('Are you sure you want to delete this lesson? This action cannot be undone.')) {
                return;
            }
            // TODO: Implement lesson deletion
            showNotification('Lesson deletion - Coming soon!', false);
        }

        init();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
