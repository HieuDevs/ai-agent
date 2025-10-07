package agents

import (
	"ai-agent/utils"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"ai-agent/work-flows/services"
	"encoding/json"
	"fmt"
	"strings"
)

type AssessmentAgent struct {
	name        string
	client      client.Client
	language    string
	model       string
	temperature float64
	maxTokens   int
	config      *utils.AssessmentPromptConfig
}

type AssessmentResponse struct {
	Level                 string   `json:"level"`
	GeneralSkills         string   `json:"general_skills"`
	GrammarTips           []string `json:"grammar_tips"`
	VocabularyTips        []string `json:"vocabulary_tips"`
	FluencySuggestions    []string `json:"fluency_suggestions"`
	VocabularySuggestions []string `json:"vocabulary_suggestions"`
}

type TipObject struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type FluencySuggestion struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Phrases     []string `json:"phrases"`
}

type VocabSuggestion struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Vocab       []string `json:"vocab"`
}

func NewAssessmentAgent(
	client client.Client,
	language string,
) *AssessmentAgent {
	if language == "" {
		language = "English"
	}

	config, err := utils.LoadAssessmentConfig()
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to load assessment config: %v", err))
		config = nil
	}

	model := "openai/gpt-4o-mini"
	temperature := 0.2
	maxTokens := 800

	if config != nil {
		if config.AssessmentAgent.LLM.Model != "" {
			model = config.AssessmentAgent.LLM.Model
		}
		if config.AssessmentAgent.LLM.Temperature > 0 {
			temperature = config.AssessmentAgent.LLM.Temperature
		}
		if config.AssessmentAgent.LLM.MaxTokens > 0 {
			maxTokens = config.AssessmentAgent.LLM.MaxTokens
		}
	}

	return &AssessmentAgent{
		name:        "AssessmentAgent",
		client:      client,
		language:    language,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		config:      config,
	}
}

func (aa *AssessmentAgent) Name() string {
	return aa.name
}

func (aa *AssessmentAgent) Capabilities() []string {
	return []string{
		"proficiency_assessment",
		"level_determination",
		"learning_tips_generation",
		"conversation_analysis",
	}
}

func (aa *AssessmentAgent) CanHandle(task string) bool {
	return strings.Contains(strings.ToLower(task), "assess") ||
		strings.Contains(strings.ToLower(task), "level") ||
		strings.Contains(strings.ToLower(task), "proficiency") ||
		strings.Contains(strings.ToLower(task), "evaluation")
}

func (aa *AssessmentAgent) GetDescription() string {
	return "Analyzes conversation history to assess learner proficiency level and provide learning tips"
}

func (aa *AssessmentAgent) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("AssessmentAgent processing task: %s", task.Task))

	historyManager, ok := task.Metadata.(*services.ConversationHistoryManager)
	if !ok {
		return &models.JobResponse{
			AgentName: aa.Name(),
			Success:   false,
			Result:    "",
			Error:     "Invalid metadata: ConversationHistoryManager required",
		}
	}

	return aa.generateAssessment(historyManager)
}

func (aa *AssessmentAgent) generateAssessment(historyManager *services.ConversationHistoryManager) *models.JobResponse {
	conversationHistory := historyManager.GetConversationHistory()

	if len(conversationHistory) == 0 {
		return &models.JobResponse{
			AgentName: aa.Name(),
			Success:   false,
			Result:    "",
			Error:     "No conversation history available for assessment",
		}
	}

	filteredHistory := aa.filterHistoryForAssessment(conversationHistory)

	if len(filteredHistory) == 0 {
		return &models.JobResponse{
			AgentName: aa.Name(),
			Success:   false,
			Result:    "",
			Error:     "No relevant messages found for assessment",
		}
	}

	utils.PrintInfo(fmt.Sprintf("Analyzing %d messages for assessment", len(filteredHistory)))

	systemPrompt := aa.buildAssessmentPrompt()
	userPrompt := aa.buildUserPrompt(filteredHistory)

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

	responseFormat := aa.buildResponseFormat()
	response := aa.getResponseWithFormat(messages, responseFormat)

	if response == "" {
		return &models.JobResponse{
			AgentName: aa.Name(),
			Success:   false,
			Result:    "",
			Error:     "Failed to generate assessment",
		}
	}

	return &models.JobResponse{
		AgentName: aa.Name(),
		Success:   true,
		Result:    response,
	}
}

func (aa *AssessmentAgent) filterHistoryForAssessment(history []models.Message) []models.Message {
	var filtered []models.Message

	for _, msg := range history {
		if msg.Role == models.MessageRoleAssistant || msg.Role == models.MessageRoleUser {
			filteredMsg := models.Message{
				Index:   msg.Index,
				Role:    msg.Role,
				Content: msg.Content,
			}

			if msg.Role == models.MessageRoleUser && msg.Evaluation != nil {
				filteredMsg.Evaluation = msg.Evaluation
			}

			filtered = append(filtered, filteredMsg)
		}
	}

	return filtered
}

func (aa *AssessmentAgent) buildAssessmentPrompt() string {
	if aa.config == nil {
		return aa.buildDefaultPrompt()
	}

	basePrompt := aa.config.AssessmentAgent.BasePrompt
	if basePrompt == "" {
		return aa.buildDefaultPrompt()
	}

	return basePrompt
}

func (aa *AssessmentAgent) buildUserPrompt(history []models.Message) string {
	historyText := aa.formatHistoryForPrompt(history)

	if aa.config == nil || aa.config.AssessmentAgent.UserPromptTemplate == "" {
		return fmt.Sprintf(`Analyze this conversation history and provide a comprehensive assessment:

Conversation History:
%s

Provide assessment with:
1. **Level**: Current CEFR level (A1, A2, B1, B2, C1, C2)
2. **General Skills**: What the learner can do at this level (in %s, maximum 10 words, be concise and specific about conversation topics)
3. **Grammar Tips**: List of 2-4 strings, each formatted as: <t>title</t><d>description</d>
   - title: Short description of which tense/grammar to use in which context
   - description: Detailed explanation of usage with examples
4. **Vocabulary Tips**: List of 2-4 strings, each formatted as: <t>title</t><d>description</d>
   - title: Short description of which vocabulary to use in which context
   - description: Detailed explanation of usage with examples
5. **Fluency Suggestions**: List of 2-5 strings, each formatted as: <t>title</t><d>description</d><s>phrase1</s><s>phrase2</s>
   - title: Short description of fluency improvement area
   - description: Explanation of what phrases to learn and why
   - phrases: List of useful phrases wrapped in <s></s> tags
6. **Vocabulary Suggestions**: List of 2-5 strings, each formatted as: <t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v>
   - title: Short description of vocabulary improvement area
   - description: Explanation of what vocabulary to learn and why
   - vocab: List of useful vocabulary words wrapped in <v></v> tags (minimum 4 words required)

Assessment Guidelines:
- Be specific and actionable
- Reference actual examples from the conversation
- Provide encouragement alongside constructive feedback
- Focus on the most important areas for improvement
- Consider the learner's consistency across multiple interactions
- For General Skills: Write in target language, maximum 10 words, be concise and specific about conversation topics discussed
- For Grammar/Vocabulary Tips: Write in target language, provide context and examples
- For Fluency Suggestions: Write in target language, provide useful phrases for natural conversation
- For Vocabulary Suggestions: Write in target language, provide relevant vocabulary words`, historyText, aa.language)
	}

	template := aa.config.AssessmentAgent.UserPromptTemplate
	template = strings.ReplaceAll(template, "{conversation_history}", historyText)
	template = strings.ReplaceAll(template, "{language}", aa.language)

	return template
}

func (aa *AssessmentAgent) formatHistoryForPrompt(history []models.Message) string {
	var builder strings.Builder

	for _, msg := range history {
		builder.WriteString(fmt.Sprintf("Message %d (%s): %s\n", msg.Index, msg.Role, msg.Content))

		if msg.Role == models.MessageRoleUser && msg.Evaluation != nil {
			builder.WriteString(fmt.Sprintf("  Evaluation: %s - %s\n", msg.Evaluation.Status, msg.Evaluation.ShortDescription))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

func (aa *AssessmentAgent) buildDefaultPrompt() string {
	return `You are an expert English language assessment specialist. Your role is to analyze a learner's conversation history and provide a comprehensive proficiency assessment with actionable learning tips.

Your assessment should include:
1. **Level Assessment**: Determine the learner's current proficiency level (A1, A2, B1, B2, C1, C2)
2. **General Skills Evaluation**: Describe what the learner can do at their current level
3. **Learning Tips**: Provide specific, actionable tips for improvement

Assessment Process:
1. Analyze the conversation history (AI messages, user messages, and evaluations)
2. Look for patterns in grammar usage, vocabulary range, sentence complexity
3. Consider consistency and accuracy across multiple interactions
4. Factor in the evaluations provided for each user message
5. Determine the most appropriate CEFR level

Be encouraging and constructive. Focus on the learner's strengths while identifying areas for growth.`
}

func (aa *AssessmentAgent) buildResponseFormat() *models.ResponseFormat {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"level": map[string]any{
				"type":        "string",
				"enum":        []string{"A1", "A2", "B1", "B2", "C1", "C2"},
				"description": "The learner's current CEFR proficiency level",
			},
			"general_skills": map[string]any{
				"type":        "string",
				"description": "Description of what the learner can do at their current level (in target language, maximum 10 words, concise and specific about conversation topics)",
			},
			"grammar_tips": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of 2-4 grammar improvement tips, each formatted as: <t>title</t><d>description</d> (multiple tags supported)",
			},
			"vocabulary_tips": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of 2-4 vocabulary expansion tips, each formatted as: <t>title</t><d>description</d> (multiple tags supported)",
			},
			"fluency_suggestions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of 2-5 fluency improvement suggestions, each formatted as: <t>title</t><d>description</d><s>phrase1</s><s>phrase2</s> (multiple tags supported)",
			},
			"vocabulary_suggestions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of 2-5 vocabulary improvement suggestions, each formatted as: <t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v> (minimum 4 vocab words required, multiple tags supported)",
			},
		},
		"required":             []string{"level", "general_skills", "grammar_tips", "vocabulary_tips", "fluency_suggestions", "vocabulary_suggestions"},
		"additionalProperties": false,
	}

	return &models.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &models.JSONSchemaSpec{
			Name:   "assessment_response",
			Strict: true,
			Schema: schema,
		},
	}
}

func (aa *AssessmentAgent) getResponseWithFormat(messages []models.Message, responseFormat *models.ResponseFormat) string {
	response, err := aa.client.ChatCompletionWithFormat(aa.model, aa.temperature, aa.maxTokens, messages, responseFormat)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to get assessment response: %v", err))
		return ""
	}
	return response
}

func (aa *AssessmentAgent) DisplayAssessment(jsonResponse string) {
	var assessment AssessmentResponse

	cleanJSON := strings.TrimSpace(jsonResponse)
	if after, ok := strings.CutPrefix(cleanJSON, "```json"); ok {
		cleanJSON = after
	} else if after, ok := strings.CutPrefix(cleanJSON, "```"); ok {
		cleanJSON = after
	}
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	err := json.Unmarshal([]byte(cleanJSON), &assessment)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to parse assessment: %v", err))
		return
	}

	levelEmoji := map[string]string{
		"A1": "🌱",
		"A2": "🌿",
		"B1": "🌳",
		"B2": "🏔️",
		"C1": "⭐",
		"C2": "👑",
	}

	emoji := levelEmoji[assessment.Level]
	if emoji == "" {
		emoji = "📊"
	}

	fmt.Println("\n📊 Proficiency Assessment:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("%s Level: %s\n\n", emoji, assessment.Level)
	fmt.Printf("🎯 General Skills:\n%s\n\n", assessment.GeneralSkills)
	fmt.Printf("📚 Grammar Tips:\n")
	for _, tip := range assessment.GrammarTips {
		tipObjects := aa.parseTaggedString(tip)
		for _, tipObj := range tipObjects {
			if tipObj.Title != "" {
				fmt.Printf("• %s\n", tipObj.Title)
			}
			if tipObj.Description != "" {
				fmt.Printf("  %s\n", tipObj.Description)
			}
		}
	}
	fmt.Printf("\n📖 Vocabulary Tips:\n")
	for _, tip := range assessment.VocabularyTips {
		tipObjects := aa.parseTaggedString(tip)
		for _, tipObj := range tipObjects {
			if tipObj.Title != "" {
				fmt.Printf("• %s\n", tipObj.Title)
			}
			if tipObj.Description != "" {
				fmt.Printf("  %s\n", tipObj.Description)
			}
		}
	}

	fmt.Printf("\n🗣️ Fluency Suggestions:\n")
	for _, tip := range assessment.FluencySuggestions {
		suggestions := aa.parseFluencySuggestion(tip)
		for _, suggestion := range suggestions {
			if suggestion.Title != "" {
				fmt.Printf("• %s\n", suggestion.Title)
			}
			if suggestion.Description != "" {
				fmt.Printf("  %s\n", suggestion.Description)
			}
			if len(suggestion.Phrases) > 0 {
				fmt.Printf("  Phrases: %s\n", strings.Join(suggestion.Phrases, ", "))
			}
		}
	}

	fmt.Printf("\n📚 Vocabulary Suggestions:\n")
	for _, tip := range assessment.VocabularySuggestions {
		suggestions := aa.parseVocabSuggestion(tip)
		for _, suggestion := range suggestions {
			if suggestion.Title != "" {
				fmt.Printf("• %s\n", suggestion.Title)
			}
			if suggestion.Description != "" {
				fmt.Printf("  %s\n", suggestion.Description)
			}
			if len(suggestion.Vocab) > 0 {
				fmt.Printf("  Vocabulary: %s\n", strings.Join(suggestion.Vocab, ", "))
			}
		}
	}
	fmt.Println("────────────────────────────────────────")
}

func ParseAssessmentResponse(jsonResponse string) (*AssessmentResponse, error) {
	var assessment AssessmentResponse

	cleanJSON := strings.TrimSpace(jsonResponse)
	if after, ok := strings.CutPrefix(cleanJSON, "```json"); ok {
		cleanJSON = after
	} else if after, ok := strings.CutPrefix(cleanJSON, "```"); ok {
		cleanJSON = after
	}
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	err := json.Unmarshal([]byte(cleanJSON), &assessment)
	if err != nil {
		return nil, fmt.Errorf("failed to parse assessment response: %w", err)
	}

	return &assessment, nil
}

func (aa *AssessmentAgent) parseTaggedString(taggedString string) []TipObject {
	var tipObjects []TipObject

	// Extract all title-description pairs
	remaining := taggedString
	for {
		// Find the next title tag
		titleStart := strings.Index(remaining, "<t>")
		if titleStart == -1 {
			break
		}

		titleEnd := strings.Index(remaining[titleStart+3:], "</t>")
		if titleEnd == -1 {
			break
		}

		titleContent := remaining[titleStart+3 : titleStart+3+titleEnd]

		// Find the corresponding description tag
		descStart := strings.Index(remaining[titleStart+3+titleEnd+4:], "<d>")
		if descStart == -1 {
			// If no description found, create object with just title
			tipObjects = append(tipObjects, TipObject{
				Title:       titleContent,
				Description: "",
			})
			remaining = remaining[titleStart+3+titleEnd+4:]
			continue
		}

		descEnd := strings.Index(remaining[titleStart+3+titleEnd+4+descStart+3:], "</d>")
		if descEnd == -1 {
			// If no closing description tag, create object with just title
			tipObjects = append(tipObjects, TipObject{
				Title:       titleContent,
				Description: "",
			})
			remaining = remaining[titleStart+3+titleEnd+4:]
			continue
		}

		descContent := remaining[titleStart+3+titleEnd+4+descStart+3 : titleStart+3+titleEnd+4+descStart+3+descEnd]

		// Create tip object
		tipObjects = append(tipObjects, TipObject{
			Title:       titleContent,
			Description: descContent,
		})

		// Move past this title-description pair
		remaining = remaining[titleStart+3+titleEnd+4+descStart+3+descEnd+4:]
	}

	// Fallback: if no tags found, create object with the whole string as description
	if len(tipObjects) == 0 {
		tipObjects = append(tipObjects, TipObject{
			Title:       "",
			Description: taggedString,
		})
	}

	return tipObjects
}

func (aa *AssessmentAgent) parseFluencySuggestion(taggedString string) []FluencySuggestion {
	var suggestions []FluencySuggestion

	// Extract all title-description-phrases groups
	remaining := taggedString
	for {
		// Find the next title tag
		titleStart := strings.Index(remaining, "<t>")
		if titleStart == -1 {
			break
		}

		titleEnd := strings.Index(remaining[titleStart+3:], "</t>")
		if titleEnd == -1 {
			break
		}

		titleContent := remaining[titleStart+3 : titleStart+3+titleEnd]

		// Find the corresponding description tag
		descStart := strings.Index(remaining[titleStart+3+titleEnd+4:], "<d>")
		if descStart == -1 {
			break
		}

		descEnd := strings.Index(remaining[titleStart+3+titleEnd+4+descStart+3:], "</d>")
		if descEnd == -1 {
			break
		}

		descContent := remaining[titleStart+3+titleEnd+4+descStart+3 : titleStart+3+titleEnd+4+descStart+3+descEnd]

		// Extract all phrases from <s></s> tags
		var phrases []string
		phraseRemaining := remaining[titleStart+3+titleEnd+4+descStart+3+descEnd+4:]
		for {
			phraseStart := strings.Index(phraseRemaining, "<s>")
			if phraseStart == -1 {
				break
			}

			phraseEnd := strings.Index(phraseRemaining[phraseStart+3:], "</s>")
			if phraseEnd == -1 {
				break
			}

			phraseContent := phraseRemaining[phraseStart+3 : phraseStart+3+phraseEnd]
			phrases = append(phrases, phraseContent)
			phraseRemaining = phraseRemaining[phraseStart+3+phraseEnd+4:]
		}

		// Create fluency suggestion
		suggestions = append(suggestions, FluencySuggestion{
			Title:       titleContent,
			Description: descContent,
			Phrases:     phrases,
		})

		// Move past this title-description-phrases group
		remaining = remaining[titleStart+3+titleEnd+4+descStart+3+descEnd+4:]
	}

	// Fallback: if no tags found, create suggestion with the whole string as description
	if len(suggestions) == 0 {
		suggestions = append(suggestions, FluencySuggestion{
			Title:       "",
			Description: taggedString,
			Phrases:     []string{},
		})
	}

	return suggestions
}

func (aa *AssessmentAgent) parseVocabSuggestion(taggedString string) []VocabSuggestion {
	var suggestions []VocabSuggestion

	// Extract all title-description-vocab groups
	remaining := taggedString
	for {
		// Find the next title tag
		titleStart := strings.Index(remaining, "<t>")
		if titleStart == -1 {
			break
		}

		titleEnd := strings.Index(remaining[titleStart+3:], "</t>")
		if titleEnd == -1 {
			break
		}

		titleContent := remaining[titleStart+3 : titleStart+3+titleEnd]

		// Find the corresponding description tag
		descStart := strings.Index(remaining[titleStart+3+titleEnd+4:], "<d>")
		if descStart == -1 {
			break
		}

		descEnd := strings.Index(remaining[titleStart+3+titleEnd+4+descStart+3:], "</d>")
		if descEnd == -1 {
			break
		}

		descContent := remaining[titleStart+3+titleEnd+4+descStart+3 : titleStart+3+titleEnd+4+descStart+3+descEnd]

		// Extract all vocab from <v></v> tags
		var vocab []string
		vocabRemaining := remaining[titleStart+3+titleEnd+4+descStart+3+descEnd+4:]
		for {
			vocabStart := strings.Index(vocabRemaining, "<v>")
			if vocabStart == -1 {
				break
			}

			vocabEnd := strings.Index(vocabRemaining[vocabStart+3:], "</v>")
			if vocabEnd == -1 {
				break
			}

			vocabContent := vocabRemaining[vocabStart+3 : vocabStart+3+vocabEnd]
			vocab = append(vocab, vocabContent)
			vocabRemaining = vocabRemaining[vocabStart+3+vocabEnd+4:]
		}

		// Create vocab suggestion
		suggestions = append(suggestions, VocabSuggestion{
			Title:       titleContent,
			Description: descContent,
			Vocab:       vocab,
		})

		// Move past this title-description-vocab group
		remaining = remaining[titleStart+3+titleEnd+4+descStart+3+descEnd+4:]
	}

	// Fallback: if no tags found, create suggestion with the whole string as description
	if len(suggestions) == 0 {
		suggestions = append(suggestions, VocabSuggestion{
			Title:       "",
			Description: taggedString,
			Vocab:       []string{},
		})
	}

	return suggestions
}
