package api_open_router

import (
	"fmt"

	"github.com/fatih/color"
)

func PrintHeader() {
	cyan := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)

	cyan.Println("╔══════════════════════════════════════════════════════════════╗")
	cyan.Println("║                    OpenRouter-Command                        ║")
	cyan.Println("║                   AI Agent CLI Tool                         ║")
	cyan.Println("╚══════════════════════════════════════════════════════════════╝")
	yellow.Println()
}

func PrintMenu() {
	green := color.New(color.FgGreen, color.Bold)
	white := color.New(color.FgWhite)

	green.Println("┌─ Available Commands ────────────────────────────────────────┐")
	white.Println("│ 1. Check API Key Status                                     │")
	white.Println("│ 2. List Available User Models                               │")
	white.Println("│ 3. Get Model Details & Endpoints                            │")
	white.Println("│ 4. Exit                                                     │")
	green.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	yellow := color.New(color.FgYellow)
	yellow.Println("💡 Tip: Add '--o json' to any command to export response to JSON file")
	fmt.Println()
}

func PrintPrompt() {
	blue := color.New(color.FgBlue, color.Bold)
	blue.Print("OpenRouter> ")
}

func PrintGoodbye() {
	green := color.New(color.FgGreen, color.Bold)
	green.Println("Goodbye! 👋")
}
