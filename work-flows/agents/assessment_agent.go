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

const defaultPrompt = `You are an expert English language assessment specialist. Your role is to analyze a learner's conversation history and provide a comprehensive proficiency assessment with actionable learning tips.

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

const userDefaultPrompt = `Analyze this conversation history and provide a comprehensive assessment:

Conversation History:
%s

Provide assessment with:
1. **Level**: Current CEFR level (A1, A2, B1, B2, C1, C2)
2. **General Skills**: What the learner can do at this level (in %s, maximum 10 words, be concise and specific about conversation topics)
3. **Grammar Tips**: List of 2-4 strings, each formatted as: <t>title</t><d>description</d>
   - title: Short description of which tense/grammar to use in which context (in %s)
   - description: Detailed explanation of usage with examples (mix of %s for explanations and English for examples) - MUST be wrapped in <d></d> tags
4. **Vocabulary Tips**: List of 2-4 strings, each formatted as: <t>title</t><d>description</d>
   - title: Short description of which vocabulary to use in which context (in %s)
   - description: Detailed explanation of usage with examples (mix of %s for explanations and English for examples) - MUST be wrapped in <d></d> tags
5. **Fluency Suggestions**: List of 2-5 strings, each formatted as: <t>title</t><d>description</d><s>phrase1</s><s>phrase2</s>
   - title: Short description of fluency improvement area (in %s)
   - description: Explanation of what phrases to learn and why (mix of %s for explanations and English for examples) - MUST be wrapped in <d></d> tags
   - phrases: List of useful phrases wrapped in <s></s> tags (MUST be in English)
6. **Vocabulary Suggestions**: List of 2-5 strings, each formatted as: <t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v>
   - title: Short description of vocabulary improvement area (in %s)
   - description: Explanation of what vocabulary to learn and why (mix of %s for explanations and English for examples) - MUST be wrapped in <d></d> tags
   - vocab: List of useful vocabulary words wrapped in <v></v> tags (MUST be in English, minimum 4 words required)

Assessment Guidelines:
- Be specific and actionable
- Reference actual examples from the conversation
- Provide encouragement alongside constructive feedback
- Focus on the most important areas for improvement
- Consider the learner's consistency across multiple interactions
- For General Skills: Write in target language, maximum 10 words, be concise and specific about conversation topics discussed
- For Grammar/Vocabulary Tips: Write titles in target language, descriptions mix target language for explanations and English for examples
- For Fluency Suggestions: Write titles in target language, descriptions mix target language for explanations and English for examples, phrases MUST be in English
- For Vocabulary Suggestions: Write titles in target language, descriptions mix target language for explanations and English for examples, vocabulary words MUST be in English`

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
		return defaultPrompt
	}

	basePrompt := aa.config.AssessmentAgent.BasePrompt
	if basePrompt == "" {
		return defaultPrompt
	}

	return basePrompt
}

func (aa *AssessmentAgent) buildUserPrompt(history []models.Message) string {
	historyText := aa.formatHistoryForPrompt(history)

	if aa.config == nil || aa.config.AssessmentAgent.UserPromptTemplate == "" {
		return fmt.Sprintf(userDefaultPrompt, historyText, aa.language, aa.language, aa.language, aa.language, aa.language, aa.language, aa.language, aa.language, aa.language)
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
				"description": "Description of what the learner can do at their current level (in target language, concise and specific about conversation topics and themes discussed)",
			},
			"grammar_tips": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of grammar improvement tips, each formatted as: <t>title</t><d>description</d> (multiple tags supported)",
			},
			"vocabulary_tips": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of vocabulary expansion tips, each formatted as: <t>title</t><d>description</d> (multiple tags supported)",
			},
			"fluency_suggestions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of fluency improvement suggestions, each formatted as: <t>title</t><d>description</d><s>phrase1</s><s>phrase2</s> etc... (phrases MUST be in English, multiple tags supported)",
			},
			"vocabulary_suggestions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of vocabulary improvement suggestions, each formatted as: <t>title</t><d>description</d><v>vocab1</v><v>vocab2</v><v>vocab3</v><v>vocab4</v> etc... (vocab words MUST be in English, minimum 4 words required, multiple tags supported)",
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

func (aa *AssessmentAgent) GenerateAssessmentStream(historyManager *services.ConversationHistoryManager, progressChan chan<- models.AssessmentStreamResponse) {
	defer close(progressChan)

	conversationHistory := historyManager.GetConversationHistory()

	if len(conversationHistory) == 0 {
		progressChan <- models.AssessmentStreamResponse{
			Error: "No conversation history available for assessment",
		}
		return
	}

	filteredHistory := aa.filterHistoryForAssessment(conversationHistory)

	if len(filteredHistory) == 0 {
		progressChan <- models.AssessmentStreamResponse{
			Error: "No relevant messages found for assessment",
		}
		return
	}

	utils.PrintInfo(fmt.Sprintf("Analyzing %d messages for assessment", len(filteredHistory)))

	// Send progress events for different phases
	progressChan <- models.AssessmentStreamResponse{
		ProgressEvent: &models.AssessmentProgressEvent{
			Type:     "level_assessment",
			Message:  "Đang đánh giá cấp độ ngôn ngữ...",
			Progress: 10,
		},
	}

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

	// Use streaming with format
	streamResponseChan := make(chan models.StreamResponse, 100)
	doneChan := make(chan bool)

	go aa.client.ChatCompletionWithFormatStream(aa.model, aa.temperature, aa.maxTokens, messages, responseFormat, streamResponseChan, doneChan)

	var fullResponse strings.Builder
	var progressTracker int = 10
	var lastProgressUpdate int = 10

	streaming := true
	for streaming {
		select {
		case streamResp := <-streamResponseChan:
			if streamResp.Error != "" {
				progressChan <- models.AssessmentStreamResponse{
					Error: streamResp.Error,
				}
				return
			}

			if len(streamResp.Choices) > 0 && streamResp.Choices[0].Delta.Content != "" {
				fullResponse.WriteString(streamResp.Choices[0].Delta.Content)

				// Update progress based on response content analysis
				currentLength := fullResponse.Len()
				if currentLength > 0 {
					// Estimate progress based on response length and content
					progressTracker = aa.estimateProgressFromContent(fullResponse.String())

					// Send progress updates at key milestones
					if progressTracker >= 30 && lastProgressUpdate < 30 {
						progressChan <- models.AssessmentStreamResponse{
							ProgressEvent: &models.AssessmentProgressEvent{
								Type:     "skills_evaluation",
								Message:  "Đang đánh giá kỹ năng tổng quát...",
								Progress: 30,
							},
						}
						lastProgressUpdate = 30
					} else if progressTracker >= 50 && lastProgressUpdate < 50 {
						progressChan <- models.AssessmentStreamResponse{
							ProgressEvent: &models.AssessmentProgressEvent{
								Type:     "grammar_tips",
								Message:  "Đang phân tích ngữ pháp...",
								Progress: 50,
							},
						}
						lastProgressUpdate = 50
					} else if progressTracker >= 70 && lastProgressUpdate < 70 {
						progressChan <- models.AssessmentStreamResponse{
							ProgressEvent: &models.AssessmentProgressEvent{
								Type:     "vocabulary_tips",
								Message:  "Đang đánh giá từ vựng...",
								Progress: 70,
							},
						}
						lastProgressUpdate = 70
					} else if progressTracker >= 85 && lastProgressUpdate < 85 {
						progressChan <- models.AssessmentStreamResponse{
							ProgressEvent: &models.AssessmentProgressEvent{
								Type:     "fluency_suggestions",
								Message:  "Đang tạo gợi ý cải thiện độ trôi chảy...",
								Progress: 85,
							},
						}
						lastProgressUpdate = 85
					} else if progressTracker >= 95 && lastProgressUpdate < 95 {
						progressChan <- models.AssessmentStreamResponse{
							ProgressEvent: &models.AssessmentProgressEvent{
								Type:     "vocabulary_suggestions",
								Message:  "Đang tạo gợi ý từ vựng...",
								Progress: 95,
							},
						}
						lastProgressUpdate = 95
					}
				}
			}

			if len(streamResp.Choices) > 0 && streamResp.Choices[0].FinishReason != nil {
				streaming = false
			}

		case <-doneChan:
			streaming = false
		}
	}

	finalResult := fullResponse.String()
	if finalResult == "" {
		progressChan <- models.AssessmentStreamResponse{
			Error: "Failed to generate assessment",
		}
		return
	}

	// Send completion event
	progressChan <- models.AssessmentStreamResponse{
		ProgressEvent: &models.AssessmentProgressEvent{
			Type:       "completed",
			Message:    "Đánh giá hoàn thành!",
			Progress:   100,
			IsComplete: true,
		},
	}

	// Send final result
	progressChan <- models.AssessmentStreamResponse{
		FinalResult: finalResult,
	}
}

func (aa *AssessmentAgent) estimateProgressFromContent(content string) int {
	// Analyze the JSON content to estimate progress
	content = strings.ToLower(content)

	// Check for different sections in the JSON response
	hasLevel := strings.Contains(content, "\"level\"")
	hasGeneralSkills := strings.Contains(content, "\"general_skills\"")
	hasGrammarTips := strings.Contains(content, "\"grammar_tips\"")
	hasVocabularyTips := strings.Contains(content, "\"vocabulary_tips\"")
	hasFluencySuggestions := strings.Contains(content, "\"fluency_suggestions\"")
	hasVocabularySuggestions := strings.Contains(content, "\"vocabulary_suggestions\"")

	// Estimate progress based on which sections are present
	if hasVocabularySuggestions {
		return 95
	} else if hasFluencySuggestions {
		return 85
	} else if hasVocabularyTips {
		return 70
	} else if hasGrammarTips {
		return 50
	} else if hasGeneralSkills {
		return 30
	} else if hasLevel {
		return 20
	}

	// Default progress based on content length
	length := len(content)
	if length > 500 {
		return 25
	} else if length > 200 {
		return 20
	} else if length > 50 {
		return 15
	}

	return 10
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

	fmt.Println("\n📊 Raw Assessment Data:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Level: %s\n", assessment.Level)
	fmt.Printf("General Skills: %s\n", assessment.GeneralSkills)
	fmt.Printf("Grammar Tips: %v\n", assessment.GrammarTips)
	fmt.Printf("Vocabulary Tips: %v\n", assessment.VocabularyTips)
	fmt.Printf("Fluency Suggestions: %v\n", assessment.FluencySuggestions)
	fmt.Printf("Vocabulary Suggestions: %v\n", assessment.VocabularySuggestions)
	fmt.Println("────────────────────────────────────────")
}
