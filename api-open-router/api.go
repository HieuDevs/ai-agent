package api_open_router

import (
	"ai-agent/utils"
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

func CheckApiKeyStatus(exportJSON bool) {
	if openRouterApiKey == "" {
		utils.PrintError("OPENROUTER_API_KEY environment variable is required")
		return
	}

	utils.PrintInfo("Checking API key status...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/key", nil)
	if err != nil {
		utils.PrintError("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("Authorization", "Bearer "+openRouterApiKey)

	resp, err := client.Do(req)
	if err != nil {
		utils.PrintError("Failed to make request: " + err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.PrintError("Failed to read response body: " + err.Error())
		return
	}

	if resp.StatusCode == http.StatusOK {
		utils.PrintSuccess("API Key is valid and working")
		cyan := color.New(color.FgCyan)
		cyan.Println("Response:")
		fmt.Println(string(body))

		if exportJSON {
			var jsonData any
			if err := json.Unmarshal(body, &jsonData); err == nil {
				utils.ExportToJSON("api_key_status", jsonData, "api_key_status", "https://openrouter.ai/api/v1/key", resp.StatusCode)
			}
		}
	} else {
		utils.PrintError(fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)))
	}
}
