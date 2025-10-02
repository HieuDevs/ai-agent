package utils

import "github.com/fatih/color"

func PrintSuccess(message string) {
	green := color.New(color.FgGreen, color.Bold)
	green.Printf("✓ %s\n", message)
}

func PrintError(message string) {
	red := color.New(color.FgRed, color.Bold)
	red.Printf("✗ %s\n", message)
}

func PrintInfo(message string) {
	yellow := color.New(color.FgYellow)
	yellow.Printf("ℹ %s\n", message)
}
