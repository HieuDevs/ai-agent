package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"ai-agent/tools"

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

	tools.Init(openRouterApiKey)
	tools.PrintHeader()

	reader := bufio.NewReader(os.Stdin)

	for {
		tools.PrintMenu()
		tools.PrintPrompt()

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		// Parse export flag from input
		cleanedInput, exportJSON := tools.ParseExportFlag(choice)

		switch cleanedInput {
		case "1":
			tools.CheckApiKeyStatus(exportJSON)
		case "2":
			tools.GetUserModels(exportJSON)
		case "3":
			fmt.Print("Enter model ID (e.g., z-ai/glm-4.6): ")
			modelID, _ := reader.ReadString('\n')
			modelID = strings.TrimSpace(modelID)
			tools.GetModelInfo(modelID, exportJSON)
		case "4":
			tools.PrintGoodbye()
			os.Exit(0)
		default:
			red := color.New(color.FgRed, color.Bold)
			red.Println("✗ Invalid choice. Please select 1-4.")
		}

		fmt.Println()
	}
}
