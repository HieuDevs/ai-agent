package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/managers"
	"ai-agent/work-flows/models"

	"github.com/fatih/color"
)

type ChatbotOrchestrator struct {
	conversationManager *managers.ConversationManager
	personalizeManager  *managers.PersonalizeManager
	sessionActive       bool
}

func NewChatbotOrchestrator(apiKey string, level models.ConversationLevel, topic string, language string) *ChatbotOrchestrator {
	sessionId := fmt.Sprintf("cli_%d", utils.GetCurrentTimestamp())

	var conversationManager *managers.ConversationManager
	if level != "" && topic != "" && language != "" {
		conversationManager = managers.NewConversationManager(apiKey, level, topic, language, sessionId)
	}

	personalizeManager := managers.NewPersonalizeManager(client.NewOpenRouterClient(apiKey))
	orchestrator := &ChatbotOrchestrator{
		conversationManager: conversationManager,
		personalizeManager:  personalizeManager,
		sessionActive:       false,
	}

	orchestrator.printWelcome()
	return orchestrator
}

func (co *ChatbotOrchestrator) printWelcome() {
	// Welcome message is now integrated into showMainMenu
}

func (co *ChatbotOrchestrator) StartConversation() {
	if co.conversationManager == nil {
		utils.PrintError("Conversation manager not initialized. Please provide level, topic, and language.")
		return
	}
	co.showMainMenu()
}

func (co *ChatbotOrchestrator) StartPersonalizeMode() {
	co.createPersonalizedLesson()
}

func (co *ChatbotOrchestrator) createPersonalizedLesson() {
	reader := bufio.NewReader(os.Stdin)
	yellow := color.New(color.FgYellow, color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)

	cyan.Println("\n📚 Create Personalized Vocabulary Lesson")
	white.Println("Let's create a custom vocabulary lesson tailored to your interests!")

	// Get topic
	white.Print("\n➤ Enter the topic you want to learn (e.g., sports, music, travel): ")
	topicInput, _ := reader.ReadString('\n')
	topic := strings.TrimSpace(topicInput)
	if topic == "" {
		topic = "general"
	}

	// Get level
	white.Println("\nSelect your English level:")
	levels := []string{"beginner", "elementary", "intermediate", "upper_intermediate", "advanced", "fluent"}
	for i, level := range levels {
		white.Printf("%d. %s\n", i+1, strings.Title(strings.ReplaceAll(level, "_", " ")))
	}

	white.Print("\n➤ Enter your level (1-6, default: intermediate): ")
	levelInput, _ := reader.ReadString('\n')
	levelStr := strings.TrimSpace(levelInput)

	var level string
	if levelStr == "" {
		level = "intermediate"
	} else {
		switch levelStr {
		case "1":
			level = levels[0]
		case "2":
			level = levels[1]
		case "3":
			level = levels[2]
		case "4":
			level = levels[3]
		case "5":
			level = levels[4]
		case "6":
			level = levels[5]
		default:
			if models.IsValidConversationLevel(levelStr) {
				level = levelStr
			} else {
				level = "intermediate"
			}
		}
	}

	// Get language
	white.Print("\n➤ Enter your native language (default: Vietnamese): ")
	languageInput, _ := reader.ReadString('\n')
	language := strings.TrimSpace(languageInput)
	if language == "" {
		language = "Vietnamese"
	}

	green.Printf("\n🎯 Creating personalized lesson for topic: %s, level: %s, language: %s\n", topic, level, language)

	// Create the lesson
	task := models.JobRequest{
		Task: "create personalized lesson detail",
		Metadata: map[string]any{
			"topic":    topic,
			"level":    level,
			"language": language,
		},
	}

	response := co.personalizeManager.ProcessTask(task)
	if response.Success {
		green.Println("\n✅ Personalized lesson created successfully!")
		fmt.Println(response.Result)
	} else {
		yellow.Printf("❌ Failed to create lesson: %s\n", response.Error)
	}

	white.Println("\nPress Enter to continue...")
	reader.ReadString('\n')
}

func (co *ChatbotOrchestrator) showMainMenu() {
	reader := bufio.NewReader(os.Stdin)
	yellow := color.New(color.FgYellow, color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)

	// Show options immediately
	cyan.Println("📋 Conversation Mode")
	white.Println("💬 Start your English conversation practice!")
	yellow.Println()

	for {
		fmt.Print("➤ Type 'start' to begin conversation, 'help' for commands, or 'quit' to exit: ")
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch strings.ToLower(choice) {
		case "start":
			green.Println("\n💬 Starting conversation...")
			co.startConversationMode()
			return
		case "quit", "exit":
			co.endSession()
			return
		case "help":
			co.showHelp()
			continue
		default:
			yellow.Println("❌ Please type 'start' to begin conversation.")
			yellow.Println("   Type 'help' for more options or 'quit' to exit.")
			continue
		}
	}
}

func (co *ChatbotOrchestrator) startConversationMode() {
	co.sessionActive = true

	conversationJob := models.JobRequest{
		Task: "conversation",
	}

	response := co.conversationManager.ProcessJob(conversationJob)
	if !response.Success {
		utils.PrintInfo(fmt.Sprintf("Failed to start conversation: %s", response.Error))
	} else {
		// Update the most recent AI message or create new one if none exists
		co.conversationManager.GetHistoryManager().UpdateLastMessage(models.MessageRoleAssistant, response.Result)

		suggestionAgent, exists := co.conversationManager.GetAgent("SuggestionAgent")
		if exists && response.Success {
			suggestionJob := models.JobRequest{
				Task:          "suggestion",
				LastAIMessage: response.Result,
			}

			suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
			if suggestionResponse.Success {
				sa := suggestionAgent.(*agents.SuggestionAgent)
				sa.DisplaySuggestions(suggestionResponse.Result)

				// Attach suggestions to the most recent AI message
				var suggestion models.SuggestionResponse
				if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestion); err == nil {
					co.conversationManager.GetHistoryManager().UpdateLastSuggestion(&suggestion)
				}
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

		if strings.ToLower(userMessage) == "assessment" {
			co.showAssessment()
			continue
		}

		if userMessage == "" {
			continue
		}

		co.processUserMessage(userMessage)
	}
}

func (co *ChatbotOrchestrator) processUserMessage(userMessage string) {

	lastAIMessage := ""
	history := co.conversationManager.GetHistoryManager().GetConversationHistory()
	if len(history) > 0 {
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == models.MessageRoleAssistant {
				lastAIMessage = history[i].Content
				break
			}
		}
	}

	// Evaluate user message and attach to exact index
	evaluateAgent, evalExists := co.conversationManager.GetAgent("EvaluateAgent")
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

			// Attach evaluation to the most recent user message
			if parsed, err := agents.ParseEvaluationResponse(evaluateResponse.Result); err == nil {
				co.conversationManager.GetHistoryManager().UpdateLastEvaluation(parsed)
			}
		}
	}

	conversationJob := models.JobRequest{
		Task:        "conversation",
		UserMessage: userMessage,
	}

	utils.PrintInfo("Processing your message...")

	conversationResponse := co.conversationManager.ProcessJob(conversationJob)
	if !conversationResponse.Success {
		utils.PrintError(fmt.Sprintf("Conversation failed: %s", conversationResponse.Error))
		return
	}

	// Generate suggestions and attach to exact AI message index
	suggestionAgent, exists := co.conversationManager.GetAgent("SuggestionAgent")
	if exists {
		suggestionJob := models.JobRequest{
			Task:          "suggestion",
			LastAIMessage: conversationResponse.Result,
		}

		suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
		if suggestionResponse.Success {
			sa := suggestionAgent.(*agents.SuggestionAgent)
			sa.DisplaySuggestions(suggestionResponse.Result)

			// Attach suggestions to the most recent AI message
			var suggestion models.SuggestionResponse
			if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestion); err == nil {
				co.conversationManager.GetHistoryManager().UpdateLastSuggestion(&suggestion)
			}
		}
	}
}

func (co *ChatbotOrchestrator) endSession() {
	co.sessionActive = false
	green := color.New(color.FgGreen, color.Bold)
	cyan := color.New(color.FgCyan)

	green.Println("\n🎉 Thank you for practicing English with me!")

	stats := co.conversationManager.GetHistoryManager().GetConversationStats()
	cyan.Printf("📈 Messages exchanged: %d (you: %d, me: %d)\n",
		stats["total_messages"], stats["user_messages"], stats["bot_messages"])
	cyan.Printf("🔑 Session ID: %s\n", co.conversationManager.GetSessionId())

	green.Println("👋 Keep practicing! See you next time!")
}

func (co *ChatbotOrchestrator) showHelp() {
	yellow := color.New(color.FgYellow, color.Bold)
	white := color.New(color.FgWhite)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	yellow.Println("\n📖 Available Commands:")
	cyan.Println("Main Menu:")
	white.Println("• start - Begin conversation practice")
	white.Println("• quit/exit - End the program")
	white.Println("• help - Show this help message")

	cyan.Println("\nConversation Mode Commands:")
	white.Println("• quit/exit - End the conversation")
	white.Println("• stats - Show conversation statistics")
	white.Println("• history - Show conversation history and export it")
	white.Println("• assessment - Show assessment of the conversation")
	white.Println("• reset - Reset conversation history")
	white.Println("• level - Show current conversation level")
	white.Println("• set level - Change conversation difficulty level")
	white.Println("• Any other text - Continue the conversation with your response")

	green.Println("\n📝 Note: All responses are in English only. We avoid sensitive or inappropriate topics.")
}

func (co *ChatbotOrchestrator) showStats() {
	stats := co.conversationManager.GetHistoryManager().GetConversationStats()

	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen)

	cyan.Println("\n📊 Conversation Statistics:")
	green.Printf("• Current level: %s\n", co.conversationManager.GetConversationAgent().GetLevel())
	green.Printf("• Total messages: %d\n", stats["total_messages"])
	green.Printf("• Your messages: %d\n", stats["user_messages"])
	green.Printf("• My responses: %d\n", stats["bot_messages"])
	green.Printf("• Session ID: %s\n", co.conversationManager.GetSessionId())
}

func (co *ChatbotOrchestrator) setLevelInteractive() {
	reader := bufio.NewReader(os.Stdin)

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	white := color.New(color.FgWhite)

	yellow.Println("\n🎯 Conversation Level Settings")
	cyan.Printf("Current level: %s\n\n", co.conversationManager.GetConversationAgent().GetLevel())

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

	co.conversationManager.GetConversationAgent().SetLevel(newLevel)

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
	currentLevel := co.conversationManager.GetConversationAgent().GetLevel()

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

	capabilities := co.conversationManager.GetConversationAgent().GetLevelSpecificCapabilities()
	white.Println("\nCapabilities:")
	for _, capability := range capabilities {
		white.Printf("• %s\n", capability)
	}

	white.Println("\nType 'set level' to change the difficulty level.")
}

func (co *ChatbotOrchestrator) resetConversation() {
	co.conversationManager.GetHistoryManager().ResetConversation()

	green := color.New(color.FgGreen)
	green.Println("🔄 Conversation history has been reset!")

	conversationJob := models.JobRequest{
		Task: "conversation",
	}

	response := co.conversationManager.ProcessJob(conversationJob)
	if !response.Success {
		utils.PrintInfo(fmt.Sprintf("Conversation reset: %s", response.Result))
	} else {
		// Update the most recent AI message or create new one if none exists
		co.conversationManager.GetHistoryManager().UpdateLastMessage(models.MessageRoleAssistant, response.Result)
		// co.manager.GetHistoryManager().EnforceMax(20)

		suggestionAgent, exists := co.conversationManager.GetAgent("SuggestionAgent")
		if exists && response.Success {
			suggestionJob := models.JobRequest{
				Task:          "suggestion",
				LastAIMessage: response.Result,
			}

			suggestionResponse := suggestionAgent.ProcessTask(suggestionJob)
			if suggestionResponse.Success {
				sa := suggestionAgent.(*agents.SuggestionAgent)
				sa.DisplaySuggestions(suggestionResponse.Result)

				// Attach suggestions to the most recent AI message
				var suggestion models.SuggestionResponse
				if err := json.Unmarshal([]byte(suggestionResponse.Result), &suggestion); err == nil {
					co.conversationManager.GetHistoryManager().UpdateLastSuggestion(&suggestion)
				}
			}
		}
	}
}

func (co *ChatbotOrchestrator) showConversationHistory() {
	history := co.conversationManager.GetHistoryManager().GetConversationHistory()

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
	cyan.Printf("Total messages: %d\n", len(history))
	cyan.Printf("Session ID: %s\n\n", co.conversationManager.GetSessionId())

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
	exportData := map[string]any{
		"session_id": co.conversationManager.GetSessionId(),
		"history":    history,
	}
	utils.ExportToJSON("conversation_history.json", exportData, "conversation_export", "/export/history", 200)
}

func (co *ChatbotOrchestrator) showAssessment() {
	assessmentAgent := co.conversationManager.GetAssessmentAgent()
	if assessmentAgent == nil {
		utils.PrintError("Assessment agent not available")
		return
	}

	historyManager := co.conversationManager.GetHistoryManager()
	if historyManager.Len() == 0 {
		yellow := color.New(color.FgYellow, color.Bold)
		yellow.Println("\n📊 Assessment")
		utils.PrintInfo("No conversation history available for assessment. Start a conversation first!")
		return
	}

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	yellow.Println("\n📊 Assessment")
	cyan.Println("Starting comprehensive assessment...")

	// Create progress channel
	progressChan := make(chan models.AssessmentStreamResponse, 100)

	// Start streaming assessment
	go assessmentAgent.GenerateAssessmentStream(historyManager, progressChan)

	// Handle progress events
	for response := range progressChan {
		if response.Error != "" {
			utils.PrintError(fmt.Sprintf("Assessment failed: %s", response.Error))
			return
		}

		if response.ProgressEvent != nil {
			event := response.ProgressEvent
			switch event.Type {
			case "level_assessment":
				cyan.Printf("🔍 %s (%d%%)\n", event.Message, event.Progress)
			case "skills_evaluation":
				cyan.Printf("📝 %s (%d%%)\n", event.Message, event.Progress)
			case "grammar_tips":
				cyan.Printf("📚 %s (%d%%)\n", event.Message, event.Progress)
			case "vocabulary_tips":
				cyan.Printf("📖 %s (%d%%)\n", event.Message, event.Progress)
			case "fluency_suggestions":
				cyan.Printf("💬 %s (%d%%)\n", event.Message, event.Progress)
			case "vocabulary_suggestions":
				cyan.Printf("🎯 %s (%d%%)\n", event.Message, event.Progress)
			case "completed":
				green.Printf("✅ %s (%d%%)\n", event.Message, event.Progress)
			}
		}

		if response.FinalResult != "" {
			fmt.Println()
			assessmentAgent.DisplayAssessment(response.FinalResult)
			break
		}
	}
}
