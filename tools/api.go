package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fatih/color"
)

var (
	openRouterApiKey string
	client           *http.Client
)

func Init(apiKey string) {
	openRouterApiKey = apiKey
	client = &http.Client{}
}

func printSuccess(message string) {
	green := color.New(color.FgGreen, color.Bold)
	green.Printf("✓ %s\n", message)
}

func printError(message string) {
	red := color.New(color.FgRed, color.Bold)
	red.Printf("✗ %s\n", message)
}

func printInfo(message string) {
	yellow := color.New(color.FgYellow)
	yellow.Printf("ℹ %s\n", message)
}

func CheckApiKeyStatus(exportJSON bool) {
	if openRouterApiKey == "" {
		printError("OPENROUTER_API_KEY environment variable is required")
		return
	}

	printInfo("Checking API key status...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/key", nil)
	if err != nil {
		printError("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("Authorization", "Bearer "+openRouterApiKey)

	resp, err := client.Do(req)
	if err != nil {
		printError("Failed to make request: " + err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		printError("Failed to read response body: " + err.Error())
		return
	}

	if resp.StatusCode == http.StatusOK {
		printSuccess("API Key is valid and working")
		cyan := color.New(color.FgCyan)
		cyan.Println("Response:")
		fmt.Println(string(body))

		if exportJSON {
			var jsonData any
			if err := json.Unmarshal(body, &jsonData); err == nil {
				ExportToJSON("api_key_status", jsonData, "api_key_status", "https://openrouter.ai/api/v1/key", resp.StatusCode)
			}
		}
	} else {
		printError(fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)))
	}
}
