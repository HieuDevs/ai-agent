package gateway

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/managers"
	"ai-agent/work-flows/models"

	"github.com/fatih/color"
)

type ChatbotOrchestrator struct {
	manager       *managers.AgentManager
	sessionActive bool
}

func NewChatbotOrchestrator(apiKey string, level models.ConversationLevel, topic string, language string) *ChatbotOrchestrator {
	manager := managers.NewManager(apiKey, level, topic, language)

	orchestrator := &ChatbotOrchestrator{
		manager:       manager,
		sessionActive: false,
	}

	orchestrator.printWelcome()
	return orchestrator
}

func (co *ChatbotOrchestrator) printWelcome() {

	yellow := color.New(color.FgYellow, color.Bold)
	green := color.New(color.FgGreen)

	green.Println("🎯 Let's start practicing! Type 'quit' to exit anytime.")
	green.Printf("📝 All responses will be in English only. We avoid sensitive or inappropriate topics.\n")
	yellow.Println()
}

func (co *ChatbotOrchestrator) StartConversation() {
	co.sessionActive = true

	conversationJob := models.JobRequest{
		Task: "conversation",
	}

	response := co.manager.ProcessJob(conversationJob)
	if !response.Success {
		utils.PrintInfo(fmt.Sprintf("Failed to start conversation: %s", response.Error))
	} else {
		suggestionAgent, exists := co.manager.GetAgent("SuggestionAgent")
		if exists && response.Success {
			suggestionJob := models.JobRequest{
				Task:          "suggestion",
				LastAIMessage: response.Result,
			}

			suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
			if suggestionResponse.Success {
				sa := suggestionAgent.(*agents.SuggestionAgent)
				sa.DisplaySuggestions(suggestionResponse.Result)
			}
		}
	}

	co.interactiveSession()
}

func (co *ChatbotOrchestrator) interactiveSession() {
	reader := bufio.NewReader(os.Stdin)

	for co.sessionActive {
		fmt.Print("\n➤ Your response: ")

		input, _ := reader.ReadString('\n')
		userMessage := strings.TrimSpace(input)

		if strings.ToLower(userMessage) == "quit" || strings.ToLower(userMessage) == "exit" {
			co.endSession()
			break
		}

		if strings.ToLower(userMessage) == "help" {
			co.showHelp()
			continue
		}

		if strings.ToLower(userMessage) == "stats" {
			co.showStats()
			continue
		}

		if strings.ToLower(userMessage) == "reset" {
			co.resetConversation()
			continue
		}

		if strings.ToLower(userMessage) == "set level" {
			co.setLevelInteractive()
			continue
		}

		if strings.ToLower(userMessage) == "level" || strings.ToLower(userMessage) == "current level" {
			co.showCurrentLevel()
			continue
		}

		if strings.ToLower(userMessage) == "history" {
			co.showConversationHistory()
			continue
		}

		if userMessage == "" {
			continue
		}

		co.processUserMessage(userMessage)
	}
}

func (co *ChatbotOrchestrator) processUserMessage(userMessage string) {
	conversationAgent := co.manager.GetConversationAgent()
	lastAIMessage := ""
	history := conversationAgent.GetConversationHistory()
	if len(history) > 0 {
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == models.MessageRoleAssistant {
				lastAIMessage = history[i].Content
				break
			}
		}
	}

	evaluateAgent, evalExists := co.manager.GetAgent("EvaluateAgent")
	if evalExists && lastAIMessage != "" {
		evaluateJob := models.JobRequest{
			Task:          "evaluate",
			UserMessage:   userMessage,
			LastAIMessage: lastAIMessage,
		}

		evaluateResponse := evaluateAgent.ProcessTask(evaluateJob)
		if evaluateResponse.Success {
			ea := evaluateAgent.(*agents.EvaluateAgent)
			ea.DisplayEvaluation(evaluateResponse.Result)
		}
	}

	conversationJob := models.JobRequest{
		Task:        "conversation",
		UserMessage: userMessage,
	}

	utils.PrintInfo("Processing your message...")

	conversationResponse := co.manager.ProcessJob(conversationJob)
	if !conversationResponse.Success {
		utils.PrintError(fmt.Sprintf("Conversation failed: %s", conversationResponse.Error))
		return
	}

	suggestionAgent, exists := co.manager.GetAgent("SuggestionAgent")
	if exists {
		suggestionJob := models.JobRequest{
			Task:          "suggestion",
			LastAIMessage: conversationResponse.Result,
		}

		suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
		if suggestionResponse.Success {
			sa := suggestionAgent.(*agents.SuggestionAgent)
			sa.DisplaySuggestions(suggestionResponse.Result)
		}
	}
}

func (co *ChatbotOrchestrator) endSession() {
	co.sessionActive = false
	conversationAgent := co.manager.GetConversationAgent()
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)

	green.Println("\n🎉 Thank you for practicing English with me!")

	stats := conversationAgent.GetConversationStats()
	cyan.Printf("📈 Messages exchanged: %d (you: %d, me: %d)\n",
		stats["total_messages"], stats["user_messages"], stats["bot_messages"])

	green.Println("👋 Keep practicing! See you next time!")
}

func (co *ChatbotOrchestrator) showHelp() {
	yellow := color.New(color.FgYellow, color.Bold)
	white := color.New(color.FgWhite)
	green := color.New(color.FgGreen)
	yellow.Println("\n📖 Available Commands:")
	white.Println("• quit/exit - End the conversation")
	white.Println("• help - Show this help message")
	white.Println("• stats - Show conversation statistics")
	white.Println("• history - Show conversation history and export it")
	white.Println("• reset - Reset conversation history")
	white.Println("• level - Show current conversation level")
	white.Println("• set level - Change conversation difficulty level")
	white.Println("• Any other text - Continue the conversation with your response")
	green.Println("\n📝 Note: All responses are in English only. We avoid sensitive or inappropriate topics.")
}

func (co *ChatbotOrchestrator) showStats() {
	conversationAgent := co.manager.GetConversationAgent()
	stats := conversationAgent.GetConversationStats()

	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen)

	cyan.Println("\n📊 Conversation Statistics:")
	green.Printf("• Current level: %s\n", conversationAgent.GetLevel())
	green.Printf("• Total messages: %d\n", stats["total_messages"])
	green.Printf("• Your messages: %d\n", stats["user_messages"])
	green.Printf("• My responses: %d\n", stats["bot_messages"])
}

func (co *ChatbotOrchestrator) setLevelInteractive() {
	reader := bufio.NewReader(os.Stdin)
	conversationAgent := co.manager.GetConversationAgent()

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	white := color.New(color.FgWhite)

	yellow.Println("\n🎯 Conversation Level Settings")
	cyan.Printf("Current level: %s\n\n", conversationAgent.GetLevel())

	green.Println("Available levels:")
	white.Println("1. Beginner      - Simple vocabulary, basic grammar, short sentences (English only, family-friendly)")
	white.Println("2. Elementary    - Basic tenses, familiar topics (English only, appropriate content)")
	white.Println("3. Intermediate  - Varied vocabulary, complex grammar (English only, respectful discussions)")
	white.Println("4. Upper Intermediate - Sophisticated language, abstract topics (English only, educational focus)")
	white.Println("5. Advanced       - Native-level vocabulary, complex discussions (English only, intellectual yet respectful)")
	white.Println("6. Fluent        - Authentic conversations as equals (English only, mature but appropriate)")

	fmt.Print("\n➤ Enter level number (1-6) or name: ")
	input, _ := reader.ReadString('\n')
	levelInput := strings.TrimSpace(input)

	if levelInput == "" {
		yellow.Println("❌ No level selected. Level unchanged.")
		return
	}

	newLevel := co.parseLevelInput(levelInput)
	if newLevel == "" {
		yellow.Println("❌ Invalid level selected. Level unchanged.")
		return
	}

	conversationAgent.SetLevel(newLevel)

	green.Printf("✅ Level changed to: %s\n", newLevel)

	currentPrompts := map[string]string{
		"beginner":           "Simple vocabulary, basic grammar, short sentences (English only, family-friendly topics)",
		"elementary":         "Basic tenses, familiar topics (English only, appropriate content)",
		"intermediate":       "Varied vocabulary, complex grammar (English only, respectful discussions)",
		"upper_intermediate": "Sophisticated language, abstract topics (English only, educational focus)",
		"advanced":           "Native-level vocabulary, complex discussions (English only, intellectual yet respectful)",
		"fluent":             "Authentic conversations as equals (English only, mature but appropriate content)",
	}

	cyan.Printf("🎓 New conversation style: %s\n", currentPrompts[string(newLevel)])

	green.Println("\nYour conversation style has been updated! Continue chatting to experience the new level.")
}

func (co *ChatbotOrchestrator) parseLevelInput(input string) models.ConversationLevel {
	input = strings.ToLower(strings.TrimSpace(input))

	levelMap := map[string]models.ConversationLevel{
		"1":                  models.ConversationLevelBeginner,
		"2":                  models.ConversationLevelElementary,
		"3":                  models.ConversationLevelIntermediate,
		"4":                  models.ConversationLevelUpperIntermediate,
		"5":                  models.ConversationLevelAdvanced,
		"6":                  models.ConversationLevelFluent,
		"beginner":           models.ConversationLevelBeginner,
		"elementary":         models.ConversationLevelElementary,
		"intermediate":       models.ConversationLevelIntermediate,
		"upper_intermediate": models.ConversationLevelUpperIntermediate,
		"upper intermediate": models.ConversationLevelUpperIntermediate,
		"advanced":           models.ConversationLevelAdvanced,
		"fluent":             models.ConversationLevelFluent,
	}

	if level, exists := levelMap[input]; exists {
		return level
	}

	return ""
}

func (co *ChatbotOrchestrator) showCurrentLevel() {
	conversationAgent := co.manager.GetConversationAgent()
	currentLevel := conversationAgent.GetLevel()

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	white := color.New(color.FgWhite)

	yellow.Println("\n🎯 Current Conversation Level")
	cyan.Printf("Level: %s\n", currentLevel)

	levelDescriptions := map[string]string{
		"beginner":           "Simple vocabulary, basic grammar, short sentences (5-8 words). English only, family-friendly topics.",
		"elementary":         "Basic tenses, familiar topics, confidence building. English responses, appropriate content.",
		"intermediate":       "Varied vocabulary, complex grammar, detailed responses. English only, respectful discussions.",
		"upper_intermediate": "Sophisticated language, abstract topics, critical thinking. English only, educational focus.",
		"advanced":           "Native-level vocabulary, complex discussions, nuanced perspectives. English only, intellectual yet respectful.",
		"fluent":             "Authentic conversations as equals, expert-level debates. English only, mature but appropriate content.",
	}

	green.Printf("Style: %s\n", levelDescriptions[string(currentLevel)])

	capabilities := conversationAgent.GetLevelSpecificCapabilities()
	white.Println("\nCapabilities:")
	for _, capability := range capabilities {
		white.Printf("• %s\n", capability)
	}

	white.Println("\nType 'set level' to change the difficulty level.")
}

func (co *ChatbotOrchestrator) resetConversation() {
	co.manager.GetConversationAgent().ResetConversation()

	green := color.New(color.FgGreen)
	green.Println("🔄 Conversation history has been reset!")

	conversationJob := models.JobRequest{
		Task: "conversation",
	}

	response := co.manager.ProcessJob(conversationJob)
	if !response.Success {
		utils.PrintInfo(fmt.Sprintf("Conversation reset: %s", response.Result))
	} else {
		suggestionAgent, exists := co.manager.GetAgent("SuggestionAgent")
		if exists && response.Success {
			suggestionJob := models.JobRequest{
				Task:        "suggestion",
				UserMessage: response.Result,
			}

			suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
			if suggestionResponse.Success {
				sa := suggestionAgent.(*agents.SuggestionAgent)
				sa.DisplaySuggestions(suggestionResponse.Result)
			}
		}
	}
}

func (co *ChatbotOrchestrator) showConversationHistory() {
	conversationAgent := co.manager.GetConversationAgent()
	history := conversationAgent.GetConversationHistory()

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	white := color.New(color.FgWhite)
	blue := color.New(color.FgBlue)

	if len(history) == 0 {
		yellow.Println("\n📜 Conversation History")
		cyan.Println("No conversation history available yet.")
		white.Println("Start a conversation to build history!")
		return
	}

	yellow.Println("\n📜 Conversation History")
	cyan.Printf("Total messages: %d\n\n", len(history))

	for i, message := range history {
		switch message.Role {
		case models.MessageRoleUser:
			green.Printf("[%d] You: %s\n", i+1, message.Content)
		case models.MessageRoleAssistant:
			blue.Printf("    AI: %s\n", message.Content)
		case models.MessageRoleSystem:
			continue
		}
	}

	white.Println()
	utils.ExportToJSON("conversation_history.json", history, "conversation_export", "/export/history", 200)
}
