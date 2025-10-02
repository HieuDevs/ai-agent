package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
)

type ExportData struct {
	Timestamp   string `json:"timestamp"`
	RequestType string `json:"request_type"`
	Endpoint    string `json:"endpoint"`
	Status      int    `json:"status"`
	Data        any    `json:"data"`
}

func sanitizeString(s string) string {
	// Loại bỏ hoặc thay thế các ký tự có thể gây lỗi
	if !utf8.ValidString(s) {
		// Thay thế ký tự không hợp lệ bằng ký tự thay thế
		return strings.ToValidUTF8(s, "?")
	}
	return s
}

func decodeUTF8(data any) any {
	switch v := data.(type) {
	case string:
		return sanitizeString(v)
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			sanitizedKey := sanitizeString(key)
			result[sanitizedKey] = decodeUTF8(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, value := range v {
			result[i] = decodeUTF8(value)
		}
		return result
	default:
		return v
	}
}

func ExportToJSON(filename string, data any, requestType, endpoint string, status int) {

	exportDir := "exports"
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		PrintError("Failed to create exports directory: " + err.Error())
		return
	}

	filepath := filepath.Join(exportDir, filename)

	// Decode UTF-8 data before export
	decodedData := decodeUTF8(data)

	exportData := ExportData{
		Timestamp:   time.Now().Format(time.RFC3339),
		RequestType: requestType,
		Endpoint:    endpoint,
		Status:      status,
		Data:        decodedData,
	}

	// Tạo JSON encoder với SetEscapeHTML(false) để không escape ký tự đặc biệt
	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(exportData); err != nil {
		PrintError("Failed to marshal JSON: " + err.Error())
		return
	}

	jsonData := []byte(strings.TrimSpace(buf.String()))

	err := os.WriteFile(filepath, jsonData, 0644)
	if err != nil {
		PrintError("Failed to write JSON file: " + err.Error())
		return
	}

	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)
	green.Printf("✓ JSON exported successfully: %s\n", filename)
	cyan.Printf("📁 File location: %s\n", filepath)
}

func ParseExportFlag(input string) (string, bool) {
	parts := strings.Fields(input)
	for i, part := range parts {
		if part == "--o" && i+1 < len(parts) && parts[i+1] == "json" {
			// Remove the --o json parts and return the cleaned input
			cleaned := strings.Join(append(parts[:i], parts[i+2:]...), " ")
			return strings.TrimSpace(cleaned), true
		}
	}
	return input, false
}
