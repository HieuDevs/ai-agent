package api_open_router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ai-agent/utils"

	"github.com/fatih/color"
)

func ListProviders(exportJSON bool) {
	if openRouterApiKey == "" {
		utils.PrintError("OPENROUTER_API_KEY environment variable is required")
		return
	}

	utils.PrintInfo("Fetching all available providers...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/providers", nil)
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
		utils.PrintSuccess("Providers list retrieved successfully")
		cyan := color.New(color.FgCyan)
		cyan.Println("Available providers:")
		fmt.Println(string(body))

		if exportJSON {
			var jsonData any
			if err := json.Unmarshal(body, &jsonData); err == nil {
				utils.ExportToJSON("list_providers", jsonData, "list_providers", "https://openrouter.ai/api/v1/providers", resp.StatusCode)
			}
		}
	} else {
		utils.PrintError(fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)))
	}
}
