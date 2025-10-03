package agents

import (
	"ai-agent/utils"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"encoding/json"
	"fmt"
	"strings"
)

type EvaluateAgent struct {
	name        string
	client      client.Client
	level       models.ConversationLevel
	topic       string
	language    string
	model       string
	temperature float64
	maxTokens   int
	config      *utils.EvaluatePromptConfig
}

type EvaluationResponse struct {
	Status           string `json:"status"`
	ShortDescription string `json:"short_description"`
	LongDescription  string `json:"long_description"`
	Correct          string `json:"correct"`
}

func NewEvaluateAgent(
	client client.Client,
	level models.ConversationLevel,
	topic string,
	language string,
) *EvaluateAgent {
	if !models.IsValidConversationLevel(string(level)) {
		level = models.ConversationLevelIntermediate
	}

	if language == "" {
		language = "English"
	}

	config, err := utils.LoadEvaluateConfig()
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to load evaluate config: %v", err))
		config = nil
	}

	model := "openai/gpt-4o-mini"
	temperature := 0.3
	maxTokens := 500

	if config != nil {
		if config.EvaluateAgent.LLM.Model != "" {
			model = config.EvaluateAgent.LLM.Model
		}
		if config.EvaluateAgent.LLM.Temperature > 0 {
			temperature = config.EvaluateAgent.LLM.Temperature
		}
		if config.EvaluateAgent.LLM.MaxTokens > 0 {
			maxTokens = config.EvaluateAgent.LLM.MaxTokens
		}
	}

	return &EvaluateAgent{
		name:        "EvaluateAgent",
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

func (ea *EvaluateAgent) Name() string {
	return ea.name
}

func (ea *EvaluateAgent) Capabilities() []string {
	return []string{
		"response_evaluation",
		"grammar_checking",
		"feedback_provision",
	}
}

func (ea *EvaluateAgent) CanHandle(task string) bool {
	return strings.Contains(strings.ToLower(task), "evaluate") ||
		strings.Contains(strings.ToLower(task), "check") ||
		strings.Contains(strings.ToLower(task), "feedback")
}

func (ea *EvaluateAgent) GetDescription() string {
	return "Evaluates learner responses and provides constructive feedback on grammar, vocabulary, and structure"
}

func (ea *EvaluateAgent) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("EvaluateAgent processing task: %s", task.Task))

	return ea.generateEvaluation(task)
}

func (ea *EvaluateAgent) generateEvaluation(task models.JobRequest) *models.JobResponse {
	userMessage := task.UserMessage
	lastAIMessage := task.LastAIMessage

	if userMessage == "" {
		return &models.JobResponse{
			AgentName: ea.Name(),
			Success:   false,
			Result:    "",
			Error:     "No user message to evaluate",
		}
	}

	utils.PrintInfo(fmt.Sprintf("Evaluating user message: %s", userMessage))

	systemPrompt := ea.buildEvaluatePrompt()
	userPrompt := ea.buildUserPrompt(userMessage, lastAIMessage)

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

	responseFormat := ea.buildResponseFormat()
	response := ea.getResponseWithFormat(messages, responseFormat)

	if response == "" {
		return &models.JobResponse{
			AgentName: ea.Name(),
			Success:   false,
			Result:    "",
			Error:     "Failed to generate evaluation",
		}
	}

	return &models.JobResponse{
		AgentName: ea.Name(),
		Success:   true,
		Result:    response,
	}
}

func (ea *EvaluateAgent) buildEvaluatePrompt() string {
	if ea.config == nil {
		return ea.buildDefaultPrompt()
	}

	basePrompt := ea.config.EvaluateAgent.BasePrompt
	if basePrompt == "" {
		return ea.buildDefaultPrompt()
	}

	guideline := ea.buildLevelGuideline()
	principles := ea.buildKeyPrinciples()

	return basePrompt + "\n\nGuidelines by level:\n\n" + guideline + "\n\n" + principles
}

func (ea *EvaluateAgent) buildLevelGuideline() string {
	if ea.config == nil {
		return ""
	}

	levelKey := string(ea.level)
	levelConfig, exists := ea.config.EvaluateAgent.LevelGuidelines[levelKey]
	if !exists {
		levelKey = string(models.ConversationLevelIntermediate)
		levelConfig = ea.config.EvaluateAgent.LevelGuidelines[levelKey]
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s:** %s\n", levelConfig.Name, levelConfig.Description))

	for _, guideline := range levelConfig.Guidelines {
		builder.WriteString(fmt.Sprintf("- %s\n", guideline))
	}

	builder.WriteString("\nEvaluation Criteria:\n")
	builder.WriteString(fmt.Sprintf("- Excellent: %s\n", levelConfig.Criteria.Excellent))
	builder.WriteString(fmt.Sprintf("- Good: %s\n", levelConfig.Criteria.Good))
	builder.WriteString(fmt.Sprintf("- Needs Improvement: %s\n", levelConfig.Criteria.NeedsImprovement))

	return builder.String()
}

func (ea *EvaluateAgent) buildKeyPrinciples() string {
	if ea.config == nil || len(ea.config.EvaluateAgent.KeyPrinciples) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Key principles:\n")

	for _, principle := range ea.config.EvaluateAgent.KeyPrinciples {
		builder.WriteString(fmt.Sprintf("- %s\n", principle))
	}

	return builder.String()
}

func (ea *EvaluateAgent) buildUserPrompt(userMessage, aiMessage string) string {
	if ea.config == nil || ea.config.EvaluateAgent.UserPromptTemplate == "" {
		return fmt.Sprintf(`Evaluate this learner's response:

User Response: "%s"
AI Question/Context: "%s"
Topic: %s
Level: %s
Target Language: %s

Provide evaluation with:
1. Status: excellent/good/needs_improvement
2. Short description: Brief encouraging feedback (in %s)
3. Long description: Detailed analysis using <b>tags</b> for highlights (in %s)
4. Correct: The corrected version in English`, userMessage, aiMessage, ea.topic, ea.level, ea.language, ea.language, ea.language)
	}

	template := ea.config.EvaluateAgent.UserPromptTemplate
	template = strings.ReplaceAll(template, "{user_message}", userMessage)
	template = strings.ReplaceAll(template, "{ai_message}", aiMessage)
	template = strings.ReplaceAll(template, "{topic}", ea.topic)
	template = strings.ReplaceAll(template, "{level}", string(ea.level))
	template = strings.ReplaceAll(template, "{language}", ea.language)

	return template
}

func (ea *EvaluateAgent) buildDefaultPrompt() string {
	return `You are an expert English learning evaluator that provides constructive, encouraging feedback.

Evaluate learner responses based on:
- Grammar accuracy
- Vocabulary usage
- Sentence structure
- Context appropriateness
- Level-appropriate complexity

Evaluation Levels:
- "excellent": Perfect or near-perfect response
- "good": Solid response with minor issues
- "needs_improvement": Noticeable errors affecting clarity

Be encouraging and constructive. Focus on helping learners improve.

Key principles:
- Always be encouraging
- Highlight what was done well first
- Be specific about errors
- Use <b>tags</b> to highlight corrections
- Provide corrected version
- Make feedback actionable`
}

func (ea *EvaluateAgent) buildResponseFormat() *models.ResponseFormat {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"excellent", "good", "needs_improvement"},
				"description": "The evaluation level",
			},
			"short_description": map[string]any{
				"type":        "string",
				"description": "Brief encouraging feedback about the response",
			},
			"long_description": map[string]any{
				"type":        "string",
				"description": "Detailed analysis with <b>tags</b> highlighting specific errors or good usage",
			},
			"correct": map[string]any{
				"type":        "string",
				"description": "The corrected version of the sentence in English (or original if already perfect)",
			},
		},
		"required":             []string{"status", "short_description", "long_description", "correct"},
		"additionalProperties": false,
	}

	return &models.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &models.JSONSchemaSpec{
			Name:   "evaluation_response",
			Strict: true,
			Schema: schema,
		},
	}
}

func (ea *EvaluateAgent) getResponseWithFormat(messages []models.Message, responseFormat *models.ResponseFormat) string {
	response, err := ea.client.ChatCompletionWithFormat(ea.model, ea.temperature, ea.maxTokens, messages, responseFormat)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to get evaluation response: %v", err))
		return ""
	}
	return response
}

func (ea *EvaluateAgent) DisplayEvaluation(jsonResponse string) {
	var evaluation EvaluationResponse

	cleanJSON := strings.TrimSpace(jsonResponse)
	if strings.HasPrefix(cleanJSON, "```json") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	} else if strings.HasPrefix(cleanJSON, "```") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	}

	err := json.Unmarshal([]byte(cleanJSON), &evaluation)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Failed to parse evaluation: %v", err))
		return
	}

	statusEmoji := map[string]string{
		"excellent":         "✨",
		"good":              "👍",
		"needs_improvement": "📚",
	}

	emoji := statusEmoji[evaluation.Status]
	if emoji == "" {
		emoji = "📝"
	}

	fmt.Println("\n📊 Evaluation:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("%s Status: %s\n\n", emoji, strings.ToUpper(evaluation.Status))
	fmt.Printf("💬 %s\n\n", evaluation.ShortDescription)

	if evaluation.LongDescription != "" {
		fmt.Printf("📖 Detailed Feedback:\n%s\n\n", evaluation.LongDescription)
	}

	if evaluation.Correct != "" && evaluation.Status != "excellent" {
		fmt.Printf("✅ Corrected: %s\n", evaluation.Correct)
	}
	fmt.Println("────────────────────────────────────────")
}

func (ea *EvaluateAgent) SetLevel(level models.ConversationLevel) {
	if !models.IsValidConversationLevel(string(level)) {
		utils.PrintError(fmt.Sprintf("Invalid level: %s", level))
		return
	}
	ea.level = level
}

func (ea *EvaluateAgent) GetLevel() models.ConversationLevel {
	return ea.level
}

func ParseEvaluationResponse(jsonResponse string) (*EvaluationResponse, error) {
	cleanJSON := strings.TrimSpace(jsonResponse)

	if strings.HasPrefix(cleanJSON, "```json") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	} else if strings.HasPrefix(cleanJSON, "```") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	}

	var evaluation EvaluationResponse
	err := json.Unmarshal([]byte(cleanJSON), &evaluation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse evaluation JSON: %w", err)
	}

	return &evaluation, nil
}
