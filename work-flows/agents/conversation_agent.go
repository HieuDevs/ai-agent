package agents

import (
	"fmt"
	"path/filepath"
	"strings"

	utils "ai-agent/utils"

	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"ai-agent/work-flows/services"
)

func getLevelSpecificPrompt(path string, level models.ConversationLevel, promptType string) string {
	_, _, fullPrompt, err := utils.GetFullPrompt(path, string(level), promptType)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Error loading prompt for level %s, type %s: %v", level, promptType, err))
		_, _, fallbackPrompt, _ := utils.GetFullPrompt(path, "intermediate", promptType)
		return fallbackPrompt
	}
	return fullPrompt
}

type ConversationAgent struct {
	name                string
	conversationHistory []models.Message
	model               string
	temperature         float64
	maxTokens           int
	Topic               string
	client              client.Client
	level               models.ConversationLevel
}

func NewConversationAgent(
	client client.Client,
	level models.ConversationLevel,
	topic string,
) *ConversationAgent {
	if !models.IsValidConversationLevel(string(level)) {
		level = models.ConversationLevelIntermediate
	}

	model, temperature, maxTokens := utils.GetLLMSettingsFromLevel(
		filepath.Join(utils.GetPromptsDir(), topic+"_prompt.yaml"),
		string(level),
	)

	return &ConversationAgent{
		name:                "ConversationAgent",
		client:              client,
		conversationHistory: []models.Message{},
		level:               level,
		Topic:               topic,
		model:               model,
		temperature:         temperature,
		maxTokens:           maxTokens,
	}
}

func (ca *ConversationAgent) Name() string {
	return ca.name
}

func (ca *ConversationAgent) Capabilities() []string {
	return []string{
		"english_conversation",
		"teaching_response",
		"conversation_starter",
		"contextual_responses",
	}
}

func (ca *ConversationAgent) CanHandle(task string) bool {
	return strings.Contains(strings.ToLower(task), "conversation") ||
		strings.Contains(strings.ToLower(task), "chat") ||
		strings.Contains(strings.ToLower(task), "talk")
}

func (ca *ConversationAgent) GetDescription() string {
	return "Handles English conversation with learners, providing appropriate responses for practice"
}

func (ca *ConversationAgent) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("ConversationAgent processing task: %s", task.Task))

	if task.UserMessage == "" {
		return ca.generateConversationStarter()
	}

	return ca.generateConversationalResponse(task, ca.model, ca.temperature, ca.maxTokens)
}

func (ca *ConversationAgent) generateConversationStarter() *models.JobResponse {
	// Get starter message from prompt
	pathPrompts := filepath.Join(utils.GetPromptsDir(), ca.Topic+"_prompt.yaml")
	starterMessage := getLevelSpecificPrompt(pathPrompts, ca.level, "starter")

	ca.addToHistory(models.MessageRoleAssistant, starterMessage)

	return &models.JobResponse{
		AgentName: ca.Name(),
		Success:   true,
		Result:    starterMessage,
	}
}

func (ca *ConversationAgent) generateConversationalResponse(
	task models.JobRequest,
	model string,
	temperature float64,
	maxTokens int,
) *models.JobResponse {
	conversationLevel := ca.level
	if task.Level != "" {
		conversationLevel = task.Level
	}
	pathPrompts := filepath.Join(utils.GetPromptsDir(), ca.Topic+"_prompt.yaml")
	levelPrompt := getLevelSpecificPrompt(pathPrompts, conversationLevel, "conversational")

	messages := []models.Message{
		{
			Role:    models.MessageRoleSystem,
			Content: levelPrompt,
		},
	}

	if len(ca.conversationHistory) > 0 {
		recentHistory := ca.getRecentHistory(6)
		messages = append(messages, recentHistory...)
	}

	messages = append(messages, models.Message{
		Role:    models.MessageRoleUser,
		Content: task.UserMessage,
	})

	fmt.Println("💬 Responding...")
	response := ca.getStreamingResponse(messages, "", model, temperature, maxTokens)

	if response == "" {
		utils.PrintError("Conversational response failed")
		return &models.JobResponse{
			AgentName: ca.Name(),
			Success:   false,
			Result:    "",
			Error:     "Failed to generate response",
		}
	}

	ca.addToHistory(models.MessageRoleUser, task.UserMessage)
	ca.addToHistory(models.MessageRoleAssistant, response)

	return &models.JobResponse{
		AgentName: ca.Name(),
		Success:   true,
		Result:    response,
	}
}

func (ca *ConversationAgent) addToHistory(role models.MessageRole, content string) {
	ca.conversationHistory = append(ca.conversationHistory, models.Message{
		Role:    models.MessageRole(role),
		Content: content,
	})

	if len(ca.conversationHistory) > 20 {
		ca.conversationHistory = ca.conversationHistory[2:]
	}
}

func (ca *ConversationAgent) getRecentHistory(maxMessages int) []models.Message {
	start := max(len(ca.conversationHistory)-maxMessages, 0)
	return ca.conversationHistory[start:]
}

func (ca *ConversationAgent) ResetConversation() {
	ca.conversationHistory = []models.Message{}
	utils.PrintSuccess("Conversation history reset")
}

func (ca *ConversationAgent) SetLevel(level models.ConversationLevel) {
	if !models.IsValidConversationLevel(string(level)) {
		utils.PrintError(fmt.Sprintf("Invalid conversation level: %s", level))
		return
	}
	ca.level = level
	utils.PrintSuccess(fmt.Sprintf("Conversation level set to: %s", level))
}

func (ca *ConversationAgent) GetLevel() models.ConversationLevel {
	return ca.level
}

func (ca *ConversationAgent) GetLevelSpecificCapabilities() []string {
	capabilities := []string{
		"english_conversation",
		"teaching_response",
		"conversation_starter",
		"contextual_responses",
		"level_appropriate_challenge",
	}

	switch ca.level {
	case models.ConversationLevelBeginner:
		capabilities = append(capabilities, "basic_vocabulary", "simple_grammar", "patient_coaching")
	case models.ConversationLevelElementary:
		capabilities = append(capabilities, "structured_learning", "confidence_building")
	case models.ConversationLevelIntermediate:
		capabilities = append(capabilities, "complex_discussion", "advanced_grammar")
	case models.ConversationLevelUpperIntermediate:
		capabilities = append(capabilities, "sophisticated_discussion", "nuanced_language")
	case models.ConversationLevelAdvanced:
		capabilities = append(capabilities, "native_level_interaction", "critical_thinking")
	case models.ConversationLevelFluent:
		capabilities = append(capabilities, "authentic_conversation", "expert_debate")
	}

	return capabilities
}

func (ca *ConversationAgent) GetConversationStats() map[string]int {
	return map[string]int{
		"total_messages": len(ca.conversationHistory),
		"user_messages":  ca.countMessagesByRole(models.MessageRoleUser),
		"bot_messages":   ca.countMessagesByRole(models.MessageRoleAssistant),
	}
}

func (ca *ConversationAgent) countMessagesByRole(role models.MessageRole) int {
	count := 0
	for _, msg := range ca.conversationHistory {
		if msg.Role == models.MessageRole(role) {
			count++
		}
	}
	return count
}

func (ca *ConversationAgent) GetFullConversationHistory() []models.Message {
	return ca.conversationHistory
}

func (ca *ConversationAgent) showVietnameseTranslation(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	fmt.Println("\n🌐 Vietnamese Translation:")
	fmt.Println("──────────────────────────")

	translation, err := services.TranslateToVietnamese(text)
	if err != nil {
		fmt.Printf("❌ Translation error: %v\n", err)
		return
	}

	fmt.Printf("🇻🇳 %s\n", translation)
	fmt.Println("──────────────────────────")
}

func (ca *ConversationAgent) getStreamingResponse(
	messages []models.Message,
	prefix string,
	model string,
	temperature float64,
	maxTokens int,
) string {
	fmt.Print(prefix)

	streamResponseChan := make(chan models.StreamResponse, 10)
	done := make(chan bool)

	go ca.client.ChatCompletionStream(model, temperature, maxTokens, messages, streamResponseChan, done)

	var fullResponse strings.Builder

	for {
		select {
		case <-done:
			fullText := fullResponse.String()
			ca.showVietnameseTranslation(fullText)
			return fullText
		case streamResponse := <-streamResponseChan:
			if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.Content != "" {
				fullResponse.WriteString(streamResponse.Choices[0].Delta.Content)
				fmt.Print(streamResponse.Choices[0].Delta.Content)
			}
		}
	}
}
