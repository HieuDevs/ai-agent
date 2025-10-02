package api_open_router

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"ai-agent/utils"

	"github.com/fatih/color"
)

func RunOpenRouterCLI(openRouterApiKey string) {
	Init(openRouterApiKey)
	PrintHeader()

	reader := bufio.NewReader(os.Stdin)

	for {
		PrintMenu()
		PrintPrompt()

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		// Parse export flag from input
		cleanedInput, exportJSON := utils.ParseExportFlag(choice)

		switch cleanedInput {
		case "1":
			CheckApiKeyStatus(exportJSON)
		case "2":
			GetUserModels(exportJSON)
		case "3":
			fmt.Print("Enter model ID (e.g., z-ai/glm-4.6): ")
			modelID, _ := reader.ReadString('\n')
			modelID = strings.TrimSpace(modelID)
			GetModelInfo(modelID, exportJSON)
		case "4":
			PrintGoodbye()
			os.Exit(0)
		default:
			red := color.New(color.FgRed, color.Bold)
			red.Println("✗ Invalid choice. Please select 1-4.")
		}

		fmt.Println()
	}
}
