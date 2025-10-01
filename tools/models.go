package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fatih/color"
)

func GetModelInfo(modelID string, exportJSON bool) {
	if openRouterApiKey == "" {
		printError("OPENROUTER_API_KEY environment variable is required")
		return
	}

	if modelID == "" {
		printError("Model ID cannot be empty")
		return
	}

	parts := strings.Split(modelID, "/")
	if len(parts) != 2 {
		printError("Model ID must be in format 'author/slug' (e.g., z-ai/glm-4.6)")
		return
	}

	author := parts[0]
	slug := parts[1]

	printInfo("Fetching detailed model information and endpoints...")

	url := fmt.Sprintf("https://openrouter.ai/api/v1/models/%s/%s/endpoints", author, slug)
	req, err := http.NewRequest("GET", url, nil)
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
		printSuccess("Model information and endpoints retrieved successfully")
		cyan := color.New(color.FgCyan)
		cyan.Printf("Detailed information for model '%s':\n", modelID)
		fmt.Println(string(body))

		if exportJSON {
			var jsonData any
			if err := json.Unmarshal(body, &jsonData); err == nil {
				ExportToJSON("model_info", jsonData, "model_info", url, resp.StatusCode)
			}
		}
	} else {
		printError(fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)))
	}
}

func GetUserModels(exportJSON bool) {
	if openRouterApiKey == "" {
		printError("OPENROUTER_API_KEY environment variable is required")
		return
	}

	printInfo("Fetching user models with provider preferences...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models/user", nil)
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
		printSuccess("User models with provider preferences retrieved successfully")
		cyan := color.New(color.FgCyan)
		cyan.Println("Your preferred models and providers:")
		fmt.Println(string(body))
		if exportJSON {
			var jsonData any
			if err := json.Unmarshal(body, &jsonData); err == nil {
				ExportToJSON("user_models", jsonData, "user_models", "https://openrouter.ai/api/v1/models/user", resp.StatusCode)
			}
		}
	} else {
		printError(fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)))
	}
}
