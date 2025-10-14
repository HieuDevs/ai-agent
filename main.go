package main

import (
	"ai-agent/utils"
	"ai-agent/work-flows/gateway"
	"ai-agent/work-flows/models"
	"bufio"
	"fmt"
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

	choice := getInterfaceChoice()

	switch choice {
	case "web":
		green.Println("🚀 Starting Web UI server...")
		green.Println("📋 You can select topic and level in the browser")
		fmt.Println()
		runChatbotWebUI(apiKey)
	case "conversation":
		green.Println("💬 Starting CLI conversation mode...")
		runChatbotConversation(apiKey)
	case "personalize":
		green.Println("📚 Starting CLI personalize mode...")
		runChatbotPersonalize(apiKey)
	}
}

func runChatbotConversation(apiKey string) {
	topic := getUserInput("sports")
	level := getConversationLevel()
	language := getLanguage()

	green := color.New(color.FgGreen)
	green.Printf("🚀 Launching conversation with topic: %s, level: %s, language: %s\n\n", topic, level, language)

	chatbot := gateway.NewChatbotOrchestrator(apiKey, models.ConversationLevel(level), topic, language)
	chatbot.StartConversation()
}

func runChatbotPersonalize(apiKey string) {
	chatbot := gateway.NewChatbotOrchestrator(apiKey, "", "", "")
	chatbot.StartPersonalizeMode()
}

func runChatbotWebUI(apiKey string) {
	chatbot := gateway.NewChatbotWeb(apiKey)
	chatbot.StartWebServer("8080")
}

func getInterfaceChoice() string {
	blue := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	blue.Println("\n🖥️  Choose your interface:")
	blue.Println("1. Web UI (Browser Interface)")
	blue.Println("2. CLI Conversation (Command Line Interface)")
	blue.Println("3. CLI Personalize (Create Vocabulary Lessons)")

	reader := bufio.NewReader(os.Stdin)

	for {
		green.Print("Enter your choice (1-3, default: Web UI): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" || input == "1" {
			yellow.Println("Using Web UI interface")
			return "web"
		}

		if input == "2" {
			yellow.Println("Using CLI conversation interface")
			return "conversation"
		}

		if input == "3" {
			yellow.Println("Using CLI personalize interface")
			return "personalize"
		}

		red := color.New(color.FgRed)
		red.Println("Invalid input. Please enter 1 for Web UI, 2 for CLI Conversation, or 3 for CLI Personalize.")
	}
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
		fmt.Println(filename)
		if strings.HasPrefix(filename, "_") {
			continue
		}
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

func getUserInput(defaultValue string) string {
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

func getLanguage() string {
	green := color.New(color.FgGreen)
	blue := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)

	languages := []string{
		"Vietnamese",
		"English",
		"Spanish",
		"French",
		"German",
		"Japanese",
		"Korean",
		"Chinese",
	}

	blue.Println("\nSelect your preferred language for instructions:")
	for i, lang := range languages {
		blue.Printf("%d. %s\n", i+1, lang)
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		green.Print("Enter your language (1-8, default: Vietnamese): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			yellow.Println("Using default language: Vietnamese")
			return "Vietnamese"
		}

		if input == "1" {
			return languages[0]
		}
		if input == "2" {
			return languages[1]
		}
		if input == "3" {
			return languages[2]
		}
		if input == "4" {
			return languages[3]
		}
		if input == "5" {
			return languages[4]
		}
		if input == "6" {
			return languages[5]
		}
		if input == "7" {
			return languages[6]
		}
		if input == "8" {
			return languages[7]
		}

		for _, lang := range languages {
			if strings.EqualFold(input, lang) {
				return lang
			}
		}

		red := color.New(color.FgRed)
		red.Println("Invalid input. Please enter a number (1-8) or the language name.")
	}
}
