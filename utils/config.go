package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var conversationPromptMemCache = make(map[string]PromptConfig)
var suggestionPromptMemCache *SuggestionPromptConfig
var evaluatePromptMemCache *EvaluatePromptConfig
var assessmentPromptMemCache *AssessmentPromptConfig

type PromptConfig struct {
	ConversationLevels map[string]LevelConfig `yaml:"conversation_levels"`
}

type SuggestionPromptConfig struct {
	SuggestionAgent SuggestionAgentConfig `yaml:"suggestion_agent"`
}

type SuggestionAgentConfig struct {
	LLM                LLMSettings                     `yaml:"llm"`
	BasePrompt         string                          `yaml:"base_prompt"`
	UserPromptTemplate string                          `yaml:"user_prompt_template"`
	LevelGuidelines    map[string]LevelGuidelineConfig `yaml:"level_guidelines"`
	KeyPrinciples      []string                        `yaml:"key_principles"`
}

type LevelGuidelineConfig struct {
	Name           string               `yaml:"name"`
	Description    string               `yaml:"description"`
	Guidelines     []string             `yaml:"guidelines"`
	ExampleLeading string               `yaml:"example_leading"`
	ExampleOptions []VocabOptionExample `yaml:"example_options"`
}

type VocabOptionExample struct {
	Text  string `yaml:"text"`
	Emoji string `yaml:"emoji"`
}

type EvaluatePromptConfig struct {
	EvaluateAgent EvaluateAgentConfig `yaml:"evaluate_agent"`
}

type AssessmentPromptConfig struct {
	AssessmentAgent AssessmentAgentConfig `yaml:"assessment_agent"`
}

type EvaluateAgentConfig struct {
	LLM                LLMSettings                    `yaml:"llm"`
	BasePrompt         string                         `yaml:"base_prompt"`
	UserPromptTemplate string                         `yaml:"user_prompt_template"`
	LevelGuidelines    map[string]EvaluateLevelConfig `yaml:"level_guidelines"`
	KeyPrinciples      []string                       `yaml:"key_principles"`
}

type EvaluateLevelConfig struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Guidelines  []string               `yaml:"guidelines"`
	Criteria    EvaluateCriteriaConfig `yaml:"criteria"`
}

type EvaluateCriteriaConfig struct {
	Excellent        string `yaml:"excellent"`
	Good             string `yaml:"good"`
	NeedsImprovement string `yaml:"needs_improvement"`
}

type AssessmentAgentConfig struct {
	LLM                LLMSettings `yaml:"llm"`
	BasePrompt         string      `yaml:"base_prompt"`
	UserPromptTemplate string      `yaml:"user_prompt_template"`
}

type LLMSettings struct {
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

type LevelConfig struct {
	Role           string      `yaml:"role"`
	Personality    string      `yaml:"personality"`
	Starter        string      `yaml:"starter"`
	Conversational string      `yaml:"conversational"`
	LLM            LLMSettings `yaml:"llm"`
}

func loadPromptsConfig(path string) (*PromptConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config PromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return &config, nil
}

func GetFullPrompt(path string, level string, promptType string) (string, string, string, error) {
	if _, exists := conversationPromptMemCache[path]; !exists {
		prompts, err := loadPromptsConfig(path)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to load prompts config: %w", err)
		}
		conversationPromptMemCache[path] = *prompts
	}

	levelConfig, exists := conversationPromptMemCache[path].ConversationLevels[level]
	if !exists {
		return "", "", "", fmt.Errorf("conversation level '%s' not found", level)
	}

	var content string
	switch promptType {
	case "starter":
		content = levelConfig.Starter
		return levelConfig.Role, levelConfig.Personality, content, nil
	case "conversational":
		content = levelConfig.Conversational
	default:
		return "", "", "", fmt.Errorf("invalid prompt type '%s'", promptType)
	}

	fullPrompt := fmt.Sprintf("Role: %s\nPersonality: %s\n\n%s",
		levelConfig.Role, levelConfig.Personality, content)

	return levelConfig.Role, levelConfig.Personality, fullPrompt, nil
}

func GetLLMSettingsFromLevel(path string, level string) (string, float64, int) {
	if _, exists := conversationPromptMemCache[path]; !exists {
		prompts, err := loadPromptsConfig(path)
		if err != nil {
			return "openai/gpt-4o-mini", 0.7, 1000
		}
		conversationPromptMemCache[path] = *prompts
	}

	levelConfig, exists := conversationPromptMemCache[path].ConversationLevels[level]
	if !exists {
		return "openai/gpt-4o-mini", 0.7, 1000
	}

	llm := levelConfig.LLM
	model := llm.Model
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	temperature := llm.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := llm.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	return model, temperature, maxTokens
}

func GetPromptsDir() string {
	dir, _ := os.Getwd()
	filePath := filepath.Join(dir, "prompts")
	return filePath
}

func LoadSuggestionConfig() (*SuggestionPromptConfig, error) {
	if suggestionPromptMemCache != nil {
		return suggestionPromptMemCache, nil
	}

	path := filepath.Join(GetPromptsDir(), "_suggestion_vocab_prompt.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("suggestion config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read suggestion config file: %w", err)
	}

	var config SuggestionPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse suggestion YAML config: %w", err)
	}

	suggestionPromptMemCache = &config
	return suggestionPromptMemCache, nil
}

func LoadEvaluateConfig() (*EvaluatePromptConfig, error) {
	if evaluatePromptMemCache != nil {
		return evaluatePromptMemCache, nil
	}

	path := filepath.Join(GetPromptsDir(), "_evaluate_prompt.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("evaluate config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read evaluate config file: %w", err)
	}

	var config EvaluatePromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse evaluate YAML config: %w", err)
	}

	evaluatePromptMemCache = &config
	return evaluatePromptMemCache, nil
}

func LoadAssessmentConfig() (*AssessmentPromptConfig, error) {
	if assessmentPromptMemCache != nil {
		return assessmentPromptMemCache, nil
	}

	path := filepath.Join(GetPromptsDir(), "_assessment_prompt.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("assessment config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read assessment config file: %w", err)
	}

	var config AssessmentPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse assessment YAML config: %w", err)
	}

	assessmentPromptMemCache = &config
	return assessmentPromptMemCache, nil
}
