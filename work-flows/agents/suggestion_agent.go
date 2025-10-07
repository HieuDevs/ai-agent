package agents

import (
	"ai-agent/utils"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"encoding/json"
	"fmt"
	"strings"
)

type SuggestionAgent struct {
	name        string
	client      client.Client
	level       models.ConversationLevel
	topic       string
	language    string
	model       string
	temperature float64
	maxTokens   int
	config      *utils.SuggestionPromptConfig
}

func NewSuggestionAgent(
	client client.Client,
	level models.ConversationLevel,
	topic string,
	language string,
) *SuggestionAgent {
	if !models.IsValidConversationLevel(string(level)) {
		level = models.ConversationLevelIntermediate
	}

	if language == "" {
		language = "English"
	}

	config, err := utils.LoadSuggestionConfig()
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to load suggestion config: %v", err))
		config = nil
	}

	model := "openai/gpt-4o-mini"
	temperature := 0.7
	maxTokens := 150

	if config != nil {
		if config.SuggestionAgent.LLM.Model != "" {
			model = config.SuggestionAgent.LLM.Model
		}
		if config.SuggestionAgent.LLM.Temperature > 0 {
			temperature = config.SuggestionAgent.LLM.Temperature
		}
		if config.SuggestionAgent.LLM.MaxTokens > 0 {
			maxTokens = config.SuggestionAgent.LLM.MaxTokens
		}
	}

	return &SuggestionAgent{
		name:        "SuggestionAgent",
		client:      client,
		level:       level,
		topic:       topic,
		language:    language,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		config:      config,
	}
}

func (sa *SuggestionAgent) Name() string {
	return sa.name
}

func (sa *SuggestionAgent) Capabilities() []string {
	return []string{
		"vocabulary_suggestion",
		"response_guidance",
		"sentence_completion",
	}
}

func (sa *SuggestionAgent) CanHandle(task string) bool {
	return strings.Contains(strings.ToLower(task), "suggestion") ||
		strings.Contains(strings.ToLower(task), "vocab") ||
		strings.Contains(strings.ToLower(task), "help")
}

func (sa *SuggestionAgent) GetDescription() string {
	return "Provides vocabulary suggestions and sentence starters to help users respond in conversations"
}

func (sa *SuggestionAgent) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("SuggestionAgent processing task: %s", task.Task))

	return sa.generateSuggestions(task)
}

func (sa *SuggestionAgent) generateSuggestions(task models.JobRequest) *models.JobResponse {
	lastMessage := task.LastAIMessage
	utils.PrintInfo(fmt.Sprintf("Last AI message: %s", lastMessage))
	systemPrompt := sa.buildSuggestionPrompt()
	userPrompt := sa.buildUserPrompt(lastMessage)

	messages := []models.Message{
		{
			Role:    models.MessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    models.MessageRoleUser,
			Content: userPrompt,
		},
	}

	responseFormat := sa.buildResponseFormat()
	response := sa.getResponseWithFormat(messages, responseFormat)

	if response == "" {
		return &models.JobResponse{
			AgentName: sa.Name(),
			Success:   false,
			Result:    "",
			Error:     "Failed to generate suggestions",
		}
	}

	return &models.JobResponse{
		AgentName: sa.Name(),
		Success:   true,
		Result:    response,
	}
}

func (sa *SuggestionAgent) buildSuggestionPrompt() string {
	if sa.config == nil {
		return sa.buildDefaultPrompt()
	}

	basePrompt := sa.config.SuggestionAgent.BasePrompt
	if basePrompt == "" {
		return sa.buildDefaultPrompt()
	}

	guideline := sa.buildLevelGuideline()
	principles := sa.buildKeyPrinciples()

	return basePrompt + "\n\nGuidelines by level:\n\n" + guideline + "\n\n" + principles
}

func (sa *SuggestionAgent) buildLevelGuideline() string {
	if sa.config == nil {
		return ""
	}

	levelKey := string(sa.level)
	levelConfig, exists := sa.config.SuggestionAgent.LevelGuidelines[levelKey]
	if !exists {
		levelKey = string(models.ConversationLevelIntermediate)
		levelConfig = sa.config.SuggestionAgent.LevelGuidelines[levelKey]
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s:** %s\n", levelConfig.Name, levelConfig.Description))

	for _, guideline := range levelConfig.Guidelines {
		builder.WriteString(fmt.Sprintf("- %s\n", guideline))
	}

	builder.WriteString(fmt.Sprintf("- Example: \"%s\"\n", levelConfig.ExampleLeading))
	builder.WriteString("  Options: ")

	for i, option := range levelConfig.ExampleOptions {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf(`{"text": "%s", "emoji": "%s"}`, option.Text, option.Emoji))
	}

	return builder.String()
}

func (sa *SuggestionAgent) buildKeyPrinciples() string {
	if sa.config == nil || len(sa.config.SuggestionAgent.KeyPrinciples) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Key principles:\n")

	for _, principle := range sa.config.SuggestionAgent.KeyPrinciples {
		builder.WriteString(fmt.Sprintf("- %s\n", principle))
	}

	return builder.String()
}

func (sa *SuggestionAgent) buildUserPrompt(lastMessage string) string {
	if sa.config == nil || sa.config.SuggestionAgent.UserPromptTemplate == "" {
		return fmt.Sprintf(`The AI just said: "%s"

Generate helpful and creative suggestions for the learner to respond naturally. Provide varied, interesting options that fit the conversation level.

Topic: %s
Level: %s
Target Language: %s

Important: 
- Translate the leading_sentence to %s to guide the learner
- Keep vocab_options text in English (for learning purposes)
- Each vocab option must include a relevant emoji that matches the meaning`, lastMessage, sa.topic, sa.level, sa.language, sa.language)
	}

	template := sa.config.SuggestionAgent.UserPromptTemplate
	template = strings.ReplaceAll(template, "{last_message}", lastMessage)
	template = strings.ReplaceAll(template, "{topic}", sa.topic)
	template = strings.ReplaceAll(template, "{level}", string(sa.level))
	template = strings.ReplaceAll(template, "{language}", sa.language)

	return template
}

func (sa *SuggestionAgent) buildDefaultPrompt() string {
	return `You are a creative English learning assistant that provides engaging vocabulary suggestions.

Your role is to help learners respond naturally by suggesting:
- A conversational leading sentence that guides their response
- 3 diverse, interesting vocabulary options with relevant emojis
- Options should be varied (different types of responses, tones, perspectives)

Make suggestions feel natural and encouraging. Be creative and flexible.

Key principles:
- Be creative and varied in your suggestions
- Match the conversation context
- Include diverse response types (agreement, disagreement, curiosity, etc.)
- Make options feel natural and usable
- Emojis should enhance meaning, not distract`
}

func (sa *SuggestionAgent) buildResponseFormat() *models.ResponseFormat {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"leading_sentence": map[string]any{
				"type":        "string",
				"description": "A brief, conversational sentence guiding how to respond",
			},
			"vocab_options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text": map[string]any{
							"type":        "string",
							"description": "The vocabulary word or phrase",
						},
						"emoji": map[string]any{
							"type":        "string",
							"description": "A relevant emoji that matches the meaning",
						},
					},
					"required":             []string{"text", "emoji"},
					"additionalProperties": false,
				},
				"description": "Exactly 3 diverse vocabulary options with emojis",
				"minItems":    3,
				"maxItems":    3,
			},
		},
		"required":             []string{"leading_sentence", "vocab_options"},
		"additionalProperties": false,
	}

	return &models.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &models.JSONSchemaSpec{
			Name:   "suggestion_response",
			Strict: true,
			Schema: schema,
		},
	}
}

func (sa *SuggestionAgent) getResponseWithFormat(messages []models.Message, responseFormat *models.ResponseFormat) string {
	response, err := sa.client.ChatCompletionWithFormat(sa.model, sa.temperature, sa.maxTokens, messages, responseFormat)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to get suggestion response: %v", err))
		return ""
	}
	return response
}

func (sa *SuggestionAgent) DisplaySuggestions(jsonResponse string) {
	var suggestion models.SuggestionResponse

	cleanJSON := strings.TrimSpace(jsonResponse)
	if after, ok := strings.CutPrefix(cleanJSON, "```json"); ok {
		cleanJSON = after
	} else if after, ok := strings.CutPrefix(cleanJSON, "```"); ok {
		cleanJSON = after
	}
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	err := json.Unmarshal([]byte(cleanJSON), &suggestion)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to parse suggestions: %v", err))
		return
	}

	fmt.Println("\n💡 Suggestions:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("📝 %s\n\n", suggestion.LeadingSentence)

	if len(suggestion.VocabOptions) > 0 {
		fmt.Println("Vocabulary options:")
		for i, vocab := range suggestion.VocabOptions {
			fmt.Printf("  %d. %s %s\n", i+1, vocab.Emoji, vocab.Text)
		}
	}
	fmt.Println("────────────────────────────────────────")
}

func (sa *SuggestionAgent) SetLevel(level models.ConversationLevel) {
	if !models.IsValidConversationLevel(string(level)) {
		utils.PrintError(fmt.Sprintf("Invalid level: %s", level))
		return
	}
	sa.level = level
}

func (sa *SuggestionAgent) GetLevel() models.ConversationLevel {
	return sa.level
}
