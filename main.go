package main

import (
	"ai-agent/utils"
	workflows "ai-agent/work-flows/agents"
	"ai-agent/work-flows/models"
	"bufio"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	openRouterApiKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterApiKey == "" {
		red := color.New(color.FgRed, color.Bold)
		yellow := color.New(color.FgYellow)
		red.Println("✗ OPENROUTER_API_KEY environment variable is required")
		yellow.Println("ℹ Please set your OpenRouter API key in the environment or .env file")
		os.Exit(1)
	}

	runEnglishChatbot(openRouterApiKey)
}

func runEnglishChatbot(apiKey string) {
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	yellow.Println("\n🎯 Starting English Conversation Chatbot...")

	topic := getUserInput("love", "Topic (e.g., travel, food, work, hobbies): ")
	level := getConversationLevel()

	green.Printf("🚀 Launching chatbot with topic: %s, level: %s\n\n", topic, level)

	runChatbotWithTopic(apiKey, models.ConversationLevel(level), topic)
}

func runChatbotWithTopic(apiKey string, level models.ConversationLevel, topic string) {
	chatbot := workflows.NewChatbotOrchestrator(apiKey, level, topic)
	chatbot.StartConversation()
}

func getAvailableTopics() []string {
	configDir := utils.GetPromptsDir()
	files, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		log.Printf("Error reading config directory: %v", err)
		return []string{"love"}
	}

	var topics []string
	for _, file := range files {
		filename := filepath.Base(file)
		if strings.HasSuffix(filename, "_prompt.yaml") {
			topic := strings.TrimSuffix(filename, "_prompt.yaml")
			if topic != "" {
				topics = append(topics, topic)
			}
		}
	}

	sort.Strings(topics)
	return topics
}

func getUserInput(defaultValue, prompt string) string {
	green := color.New(color.FgGreen)
	blue := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)

	topics := getAvailableTopics()

	blue.Println("What would you like to talk about?")
	blue.Println("\nAvailable conversation topics:")
	for _, topic := range topics {
		yellow.Printf("• %s\n", strings.Title(topic))
	}
	blue.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		green.Print(prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			input = defaultValue
			blue.Printf("Using default topic: %s\n", input)
			return input
		}

		if len(input) < 2 {
			red := color.New(color.FgRed)
			red.Println("Topic must be at least 2 characters long. Please try again.")
			continue
		}

		inputLower := strings.ToLower(input)
		for _, topic := range topics {
			if strings.ToLower(topic) == inputLower {
				return topic
			}
		}

		blue.Printf("Hmm, I don't see '%s' in the available topics. ", input)
		blue.Println("Please choose from the list above or try again.")
	}
}

func getConversationLevel() string {
	green := color.New(color.FgGreen)
	blue := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)

	levels := []string{
		"beginner",
		"elementary",
		"intermediate",
		"upper_intermediate",
		"advanced",
		"fluent",
	}

	blue.Println("\nSelect your English conversation level:")
	for i, level := range levels {
		blue.Printf("%d. %s\n", i+1, strings.Title(strings.ReplaceAll(level, "_", " ")))
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		green.Print("Enter your level (1-6, default: intermediate): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			yellow.Println("Using default level: intermediate")
			return "intermediate"
		}

		if input == "1" || strings.ToLower(input) == levels[0] {
			return levels[0]
		}
		if input == "2" || strings.ToLower(input) == levels[1] {
			return levels[1]
		}
		if input == "3" || strings.ToLower(input) == levels[2] {
			return levels[2]
		}
		if input == "4" || strings.ToLower(input) == levels[3] {
			return levels[3]
		}
		if input == "5" || strings.ToLower(input) == levels[4] {
			return levels[4]
		}
		if input == "6" || strings.ToLower(input) == levels[5] {
			return levels[5]
		}
		if models.IsValidConversationLevel(strings.ToLower(input)) {
			return strings.ToLower(input)
		}

		red := color.New(color.FgRed)
		red.Println("Invalid input. Please enter a number (1-6) or the level name.")
	}
}
