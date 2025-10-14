package models

// Message roles

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

func (r MessageRole) String() string {
	return string(r)
}

type VocabOption struct {
	Text  string `json:"text"`  // Vocabulary phrase
	Emoji string `json:"emoji"` // Relevant emoji
}

type SuggestionResponse struct {
	LeadingSentence string        `json:"leading_sentence"`
	VocabOptions    []VocabOption `json:"vocab_options"`
}

type EvaluationResponse struct {
	Status           string `json:"status"`            // excellent/good/needs_improvement
	ShortDescription string `json:"short_description"` // Brief encouraging feedback
	LongDescription  string `json:"long_description"`  // Detailed analysis with HTML tags
	Correct          string `json:"correct"`           // Corrected version in English
}

type Message struct {
	Index      int                 `json:"index"`
	Role       MessageRole         `json:"role"`
	Content    string              `json:"content"`
	Suggestion *SuggestionResponse `json:"suggestion,omitempty"` // Only for AI messages
	Evaluation *EvaluationResponse `json:"evaluation,omitempty"` // Only for user messages
}

type ConversationLevel string

const (
	ConversationLevelBeginner          ConversationLevel = "beginner"
	ConversationLevelElementary        ConversationLevel = "elementary"
	ConversationLevelIntermediate      ConversationLevel = "intermediate"
	ConversationLevelUpperIntermediate ConversationLevel = "upper_intermediate"
	ConversationLevelAdvanced          ConversationLevel = "advanced"
	ConversationLevelFluent            ConversationLevel = "fluent"
)

func (l ConversationLevel) String() string {
	return string(l)
}

func IsValidConversationLevel(level string) bool {
	switch ConversationLevel(level) {
	case ConversationLevelBeginner, ConversationLevelElementary,
		ConversationLevelIntermediate, ConversationLevelUpperIntermediate,
		ConversationLevelAdvanced, ConversationLevelFluent:
		return true
	default:
		return false
	}
}

type JobRequest struct {
	Task          string            `json:"task"`
	UserMessage   string            `json:"user_message"`
	LastAIMessage string            `json:"last_ai_message"`
	Level         ConversationLevel `json:"level,omitempty"`
	Metadata      any               `json:"metadata"`
}

type JobResponse struct {
	AgentName string `json:"agent_name"`
	Success   bool   `json:"success"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	Metadata  any    `json:"metadata,omitempty"`
}

type ResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *JSONSchemaSpec `json:"json_schema,omitempty"`
}

type JSONSchemaSpec struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type ChatRequest struct {
	Model     string   `json:"model"`
	Models    []string `json:"models,omitempty"`
	Providers struct {
		Sort string `json:"sort"`
	} `json:"providers"`
	Usage struct {
		Include bool `json:"include"`
	} `json:"usage"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens"`
	Stream         bool            `json:"stream"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type StreamResponse struct {
	ID       string `json:"id"`
	Provider string `json:"provider,omitzero"`
	Model    string `json:"model,omitzero"`
	Object   string `json:"object,omitzero"`
	Created  int64  `json:"created,omitzero"`
	Error    string `json:"error,omitzero"`
	Choices  []struct {
		Index int `json:"index,omitzero"`
		Delta struct {
			Role    string `json:"role,omitzero"`
			Content string `json:"content,omitzero"`
		} `json:"delta,omitzero"`
		FinishReason       *string `json:"finish_reason,omitzero"`
		NativeFinishReason *string `json:"native_finish_reason,omitzero"`
		Logprobs           *string `json:"logprobs,omitzero"`
	} `json:"choices,omitzero"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens,omitzero"`
		CompletionTokens    int `json:"completion_tokens,omitzero"`
		TotalTokens         int `json:"total_tokens,omitzero"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens,omitzero"`
			AudioTokens  int `json:"audio_tokens,omitzero"`
		} `json:"prompt_tokens_details,omitzero"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens,omitzero"`
		} `json:"completion_tokens_details,omitzero"`
	} `json:"usage,omitzero"`
}

type AssessmentProgressEvent struct {
	Type       string `json:"type"`        // "level_assessment", "skills_evaluation", "grammar_tips", "vocabulary_tips", "fluency_suggestions", "vocabulary_suggestions", "completed"
	Message    string `json:"message"`     // Progress message
	Progress   int    `json:"progress"`    // Progress percentage (0-100)
	IsComplete bool   `json:"is_complete"` // Whether this phase is complete
}

type AssessmentStreamResponse struct {
	ProgressEvent *AssessmentProgressEvent `json:"progress_event,omitempty"`
	FinalResult   string                   `json:"final_result,omitempty"`
	Error         string                   `json:"error,omitempty"`
}

type PersonalizeVocabItem struct {
	Vocab           string `json:"vocab"`            // English vocabulary word
	Meaning         string `json:"meaning"`          // Meaning in native language
	Sentence        string `json:"sentence"`         // Example sentence with vocab highlighted in <b>...</b>
	SentenceMeaning string `json:"sentence_meaning"` // Translation of the sentence in native language
}

type PersonalizeLessonResponse struct {
	Emoji       string                 `json:"emoji"`       // Relevant emoji for the topic
	Title       string                 `json:"title"`       // Engaging lesson title
	Description string                 `json:"description"` // Motivating lesson description
	Vocabulary  []PersonalizeVocabItem `json:"vocabulary"`  // 4 essential vocabulary items
}
