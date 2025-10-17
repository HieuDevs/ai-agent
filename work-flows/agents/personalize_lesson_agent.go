package agents

import (
	"ai-agent/utils"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	agentNamePersonalizeLesson          = "PersonalizeLessonAgent"
	defaultModelPersonalizeLesson       = "openai/gpt-4o-mini"
	defaultTemperaturePersonalizeLesson = 0.8
	defaultMaxTokensPersonalizeLesson   = 1000
	schemaNamePersonalizeLessonResponse = "personalize_lesson_response"
)

type PersonalizeLessonAgent struct {
	name        string
	client      client.Client
	model       string
	temperature float64
	maxTokens   int
	config      *utils.PersonalizeLessonPromptConfig
}

func NewPersonalizeLessonAgent(client client.Client) *PersonalizeLessonAgent {
	config, err := utils.LoadPersonalizeLessonConfig()
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to load personalize lesson config: %v", err))
		config = nil
	}

	model := defaultModelPersonalizeLesson
	temperature := defaultTemperaturePersonalizeLesson
	maxTokens := defaultMaxTokensPersonalizeLesson

	if config != nil {
		if config.PersonalizeLessonAgent.LLM.Model != "" {
			model = config.PersonalizeLessonAgent.LLM.Model
		}
		if config.PersonalizeLessonAgent.LLM.Temperature > 0 {
			temperature = config.PersonalizeLessonAgent.LLM.Temperature
		}
		if config.PersonalizeLessonAgent.LLM.MaxTokens > 0 {
			maxTokens = config.PersonalizeLessonAgent.LLM.MaxTokens
		}
	}

	return &PersonalizeLessonAgent{
		name:        agentNamePersonalizeLesson,
		client:      client,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		config:      config,
	}
}

func (pla *PersonalizeLessonAgent) Name() string {
	return pla.name
}

func (pla *PersonalizeLessonAgent) Capabilities() []string {
	return []string{
		"lesson_detail_creation",
		"personalized_learning",
		"lesson_design",
		"vocabulary_creation",
	}
}

func (pla *PersonalizeLessonAgent) CanHandle(task string) bool {
	return strings.Contains(strings.ToLower(task), "personalize") ||
		strings.Contains(strings.ToLower(task), "lesson") ||
		strings.Contains(strings.ToLower(task), "detail") ||
		strings.Contains(strings.ToLower(task), "create") ||
		strings.Contains(strings.ToLower(task), "vocabulary") ||
		strings.Contains(strings.ToLower(task), "vocab")
}

func (pla *PersonalizeLessonAgent) GetDescription() string {
	return "Creates personalized lesson details with emoji, title, description, and 4 essential vocabulary items based on user preferences"
}

func (pla *PersonalizeLessonAgent) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("PersonalizeLessonAgent processing task: %s", task.Task))

	return pla.generatePersonalizedLesson(task)
}

func (pla *PersonalizeLessonAgent) generatePersonalizedLesson(task models.JobRequest) *models.JobResponse {
	// Extract topic, level, and language from metadata
	topic, level, language := pla.extractMetadata(task.Metadata)

	systemPrompt := pla.buildPersonalizePrompt(level)
	userPrompt := pla.buildUserPrompt(topic, level, language)

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

	responseFormat := pla.buildResponseFormat()
	response := pla.getResponseWithFormat(messages, responseFormat)

	if response == "" {
		return &models.JobResponse{
			AgentName: pla.Name(),
			Success:   false,
			Result:    "",
			Error:     "Failed to generate personalized lesson",
		}
	}

	return &models.JobResponse{
		AgentName: pla.Name(),
		Success:   true,
		Result:    response,
	}
}

func (pla *PersonalizeLessonAgent) extractMetadata(metadata any) (string, models.ConversationLevel, string) {
	// Default values
	topic := "general"
	level := models.ConversationLevelIntermediate
	language := "English"

	// Try to extract from metadata if it's a map
	if metadataMap, ok := metadata.(map[string]any); ok {
		if topicVal, exists := metadataMap["topic"]; exists {
			if topicStr, ok := topicVal.(string); ok && topicStr != "" {
				topic = topicStr
			}
		}

		if levelVal, exists := metadataMap["level"]; exists {
			if levelStr, ok := levelVal.(string); ok && models.IsValidConversationLevel(levelStr) {
				level = models.ConversationLevel(levelStr)
			}
		}

		if languageVal, exists := metadataMap["language"]; exists {
			if languageStr, ok := languageVal.(string); ok && languageStr != "" {
				language = languageStr
			}
		}
	}

	return topic, level, language
}

func (pla *PersonalizeLessonAgent) buildPersonalizePrompt(level models.ConversationLevel) string {
	if pla.config == nil {
		return pla.buildDefaultPrompt()
	}

	basePrompt := pla.config.PersonalizeLessonAgent.BasePrompt
	if basePrompt == "" {
		return pla.buildDefaultPrompt()
	}

	guideline := pla.buildLevelGuideline(level)
	principles := pla.buildKeyPrinciples()

	return basePrompt + "\n\nGuidelines by level:\n\n" + guideline + "\n\n" + principles
}

func (pla *PersonalizeLessonAgent) buildLevelGuideline(level models.ConversationLevel) string {
	if pla.config == nil {
		return ""
	}

	levelKey := string(level)
	levelConfig, exists := pla.config.PersonalizeLessonAgent.LevelGuidelines[levelKey]
	if !exists {
		levelKey = string(models.ConversationLevelIntermediate)
		levelConfig = pla.config.PersonalizeLessonAgent.LevelGuidelines[levelKey]
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s:** %s\n", levelConfig.Name, levelConfig.Description))

	for _, guideline := range levelConfig.Guidelines {
		builder.WriteString(fmt.Sprintf("- %s\n", guideline))
	}

	builder.WriteString(fmt.Sprintf("- Example emoji: %s\n", levelConfig.ExampleEmoji))
	builder.WriteString(fmt.Sprintf("- Example title: \"%s\"\n", levelConfig.ExampleTitle))
	builder.WriteString(fmt.Sprintf("- Example description: \"%s\"\n", levelConfig.ExampleDescription))

	return builder.String()
}

func (pla *PersonalizeLessonAgent) buildKeyPrinciples() string {
	if pla.config == nil || len(pla.config.PersonalizeLessonAgent.KeyPrinciples) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Key principles:\n")

	for _, principle := range pla.config.PersonalizeLessonAgent.KeyPrinciples {
		builder.WriteString(fmt.Sprintf("- %s\n", principle))
	}

	return builder.String()
}

func (pla *PersonalizeLessonAgent) buildUserPrompt(topic string, level models.ConversationLevel, language string) string {
	if pla.config == nil || pla.config.PersonalizeLessonAgent.UserPromptTemplate == "" {
		return fmt.Sprintf(`Create a personalized lesson detail for:

Topic: %s
Level: %s
Native Language: %s

Generate:
1. An emoji that perfectly represents this topic
2. An engaging title that makes the learner excited to study
3. A motivating description that explains what they'll learn and why it's useful
4. 4 essential vocabulary words related to this topic and level

For each vocabulary word, provide:
- ONE clear emoji that best represents the vocabulary word (be selective and precise)
- The English word
- Its meaning in %s
- An English sentence using the word in context related to the topic, with the word highlighted between <b>...</b>
- The sentence's meaning translated into %s

Make it feel personal and tailored to their interests and proficiency level.`, topic, level, language, language, language)
	}

	template := pla.config.PersonalizeLessonAgent.UserPromptTemplate
	template = strings.ReplaceAll(template, "{topic}", topic)
	template = strings.ReplaceAll(template, "{level}", string(level))
	template = strings.ReplaceAll(template, "{language}", language)

	return template
}

func (pla *PersonalizeLessonAgent) buildDefaultPrompt() string {
	return `You are a careful lesson detail designer that creates personalized learning experiences.

Your role is to generate clear, concise lesson details based on user preferences:
- Choose ONE most relevant emoji that clearly represents the topic (be selective and precise)
- Create a short, clear title in English (under 6 words, easy to understand)
- Write a concise description in their native language (under 2 sentences, focus on practical benefits)
- Create 4 essential vocabulary words related to the topic and appropriate for the learner's level

For each vocabulary word:
- Choose ONE clear emoji that best represents the vocabulary word (be selective and precise)
- Choose English words that are essential for understanding the topic
- Format the vocabulary word as "word (type)" where type is n. = noun, v. = verb, adj. = adjective, adv. = adverb
- Provide a very short meaning in the learner's native language (2-4 words max)
- Create an English sentence that uses the word in context related to the topic
- Highlight the vocabulary word between <b>...</b> tags in the sentence
- Provide the sentence's meaning translated into the learner's native language

Be careful with emoji selection - choose the most obvious and universally understood emoji for the topic.
Keep everything simple, clear, and practical for language learners.

Key principles:
- Be careful and precise with emoji selection - choose the most obvious one for both topic and vocabulary words
- Keep titles short, clear, and easy to understand (under 6 words)
- Write concise descriptions (under 2 sentences) in the learner's native language
- Choose vocabulary words appropriate for the learner's level
- For each vocabulary word, choose ONE clear emoji that best represents the word
- Format vocabulary words as "word (type)" where type is n./v./adj./adv.
- Meanings must be very short (2-4 words max)
- Create sentences that clearly show how the word is used in context
- Focus on practical benefits and real-world application
- Make everything simple and clear for language learners
- Choose universally understood emojis that clearly represent both topic and vocabulary words`
}

func (pla *PersonalizeLessonAgent) buildResponseFormat() *models.ResponseFormat {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"emoji": map[string]any{
				"type":        "string",
				"description": "ONE clear emoji that best represents the topic (choose the most obvious one)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "A short, clear title in English (under 6 words, easy to understand)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "A concise description in the learner's native language (under 2 sentences, focus on practical benefits)",
			},
			"vocabulary": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"emoji": map[string]any{
							"type":        "string",
							"description": "ONE clear emoji that best represents the vocabulary word (be selective and precise)",
						},
						"vocab": map[string]any{
							"type":        "string",
							"description": "English vocabulary word formatted as 'word (type)' where type is n. = noun, v. = verb, adj. = adjective, adv. = adverb",
						},
						"meaning": map[string]any{
							"type":        "string",
							"description": "Very short meaning in the learner's native language (2-4 words max)",
						},
						"sentence": map[string]any{
							"type":        "string",
							"description": "English example sentence using the word in context, with the word highlighted between <b>...</b> tags",
						},
						"sentence_meaning": map[string]any{
							"type":        "string",
							"description": "Translation of the example sentence in the learner's native language",
						},
					},
					"required":             []string{"emoji", "vocab", "meaning", "sentence", "sentence_meaning"},
					"additionalProperties": false,
				},
				"minItems":    4,
				"maxItems":    4,
				"description": "Exactly 4 essential vocabulary words related to the topic and appropriate for the learner's level",
			},
		},
		"required":             []string{"emoji", "title", "description", "vocabulary"},
		"additionalProperties": false,
	}

	return &models.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &models.JSONSchemaSpec{
			Name:   schemaNamePersonalizeLessonResponse,
			Strict: true,
			Schema: schema,
		},
	}
}

func (pla *PersonalizeLessonAgent) getResponseWithFormat(messages []models.Message, responseFormat *models.ResponseFormat) string {
	response, err := pla.client.ChatCompletionWithFormat(pla.model, pla.temperature, pla.maxTokens, messages, responseFormat)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to get personalize lesson response: %v", err))
		return ""
	}
	return response
}

func (pla *PersonalizeLessonAgent) DisplayPersonalizedLesson(jsonResponse string) {
	var lesson models.PersonalizeLessonResponse

	cleanJSON := strings.TrimSpace(jsonResponse)
	if after, ok := strings.CutPrefix(cleanJSON, "```json"); ok {
		cleanJSON = after
	} else if after, ok := strings.CutPrefix(cleanJSON, "```"); ok {
		cleanJSON = after
	}
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	err := json.Unmarshal([]byte(cleanJSON), &lesson)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to parse personalized lesson: %v", err))
		return
	}

	fmt.Println("\n🎯 Personalized Lesson Detail:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("%s %s\n\n", lesson.Emoji, lesson.Title)
	fmt.Printf("📝 %s\n\n", lesson.Description)

	if len(lesson.Vocabulary) > 0 {
		fmt.Println("📚 Essential Vocabulary:")
		for i, vocab := range lesson.Vocabulary {
			fmt.Printf("%d. %s <b>%s</b> - %s\n", i+1, vocab.Emoji, vocab.Vocab, vocab.Meaning)
			fmt.Printf("   %s\n", vocab.Sentence)
			if vocab.SentenceMeaning != "" {
				fmt.Printf("   → %s\n", vocab.SentenceMeaning)
			}
			fmt.Println()
		}
	}

	fmt.Println("────────────────────────────────────────")
}
