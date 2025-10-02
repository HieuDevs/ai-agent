package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var imMemCache = make(map[string]PromptConfig)

type PromptConfig struct {
	ConversationLevels map[string]LevelConfig `yaml:"conversation_levels"`
}

type LevelConfig struct {
	Role           string `yaml:"role"`
	Personality    string `yaml:"personality"`
	Starter        string `yaml:"starter"`
	Conversational string `yaml:"conversational"`
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
	if _, exists := imMemCache[path]; !exists {
		prompts, err := loadPromptsConfig(path)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to load prompts config: %w", err)
		}
		imMemCache[path] = *prompts
	}

	levelConfig, exists := imMemCache[path].ConversationLevels[level]
	if !exists {
		return "", "", "", fmt.Errorf("conversation level '%s' not found", level)
	}

	var content string
	switch promptType {
	case "starter":
		content = levelConfig.Starter
	case "conversational":
		content = levelConfig.Conversational
	default:
		return "", "", "", fmt.Errorf("invalid prompt type '%s'", promptType)
	}

	fullPrompt := fmt.Sprintf("Role: %s\nPersonality: %s\n\n%s",
		levelConfig.Role, levelConfig.Personality, content)

	return levelConfig.Role, levelConfig.Personality, fullPrompt, nil
}

func GetPromptsDir() string {
	dir, _ := os.Getwd()
	filePath := filepath.Join(dir, "prompts")
	return filePath
}
