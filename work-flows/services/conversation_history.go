package services

import (
	"ai-agent/utils"
	"ai-agent/work-flows/models"
)

type ConversationHistoryManager struct {
	conversationHistory []models.Message
	nextIndex           int
}

func NewConversationHistoryManager() *ConversationHistoryManager {
	return &ConversationHistoryManager{
		conversationHistory: []models.Message{},
		nextIndex:           0,
	}
}

// AddMessage appends a message, assigns a stable index, and returns that index.
func (chm *ConversationHistoryManager) AddMessage(role models.MessageRole, content string) int {
	idx := chm.nextIndex
	chm.nextIndex++
	chm.conversationHistory = append(chm.conversationHistory, models.Message{
		Index:   idx,
		Role:    models.MessageRole(role),
		Content: content,
	})
	return idx
}

// UpdateLastMessage updates the most recent message of the specified role with new content
func (chm *ConversationHistoryManager) UpdateLastMessage(role models.MessageRole, content string) int {
	// Find the most recent message of the specified role
	for i := len(chm.conversationHistory) - 1; i >= 0; i-- {
		if chm.conversationHistory[i].Role == role {
			// Update the existing message
			chm.conversationHistory[i].Content = content
			return chm.conversationHistory[i].Index
		}
	}

	// If no message of this role exists, create a new one
	return chm.AddMessage(role, content)
}

// Backward compatibility for existing callers
func (chm *ConversationHistoryManager) AddToHistory(role models.MessageRole, content string) {
	chm.AddMessage(role, content)
}

// UpdateLastSuggestion updates suggestion for the most recent assistant message
func (chm *ConversationHistoryManager) UpdateLastSuggestion(suggestion *models.SuggestionResponse) {
	for i := len(chm.conversationHistory) - 1; i >= 0; i-- {
		if chm.conversationHistory[i].Role == models.MessageRoleAssistant {
			msg := chm.conversationHistory[i]
			msg.Suggestion = suggestion
			chm.conversationHistory[i] = msg
			return
		}
	}
}

// UpdateLastEvaluation updates evaluation for the most recent user message
func (chm *ConversationHistoryManager) UpdateLastEvaluation(evaluation *models.EvaluationResponse) {
	for i := len(chm.conversationHistory) - 1; i >= 0; i-- {
		if chm.conversationHistory[i].Role == models.MessageRoleUser {
			msg := chm.conversationHistory[i]
			msg.Evaluation = evaluation
			chm.conversationHistory[i] = msg
			return
		}
	}
}

func (chm *ConversationHistoryManager) GetMessageByIndex(messageIndex int) (models.Message, bool) {
	for i := len(chm.conversationHistory) - 1; i >= 0; i-- {
		if chm.conversationHistory[i].Index == messageIndex {
			return chm.conversationHistory[i], true
		}
	}
	return models.Message{}, false
}

func (chm *ConversationHistoryManager) Len() int {
	return len(chm.conversationHistory)
}

// func (chm *ConversationHistoryManager) EnforceMax(maxMessages int) {
// 	if maxMessages <= 0 {
// 		return
// 	}
// 	for len(chm.conversationHistory) > maxMessages {
// 		chm.conversationHistory = chm.conversationHistory[1:]
// 	}
// }

func (chm *ConversationHistoryManager) GetRecentHistory(maxMessages int) []models.Message {
	start := max(len(chm.conversationHistory)-maxMessages, 0)
	return chm.conversationHistory[start:]
}

func (chm *ConversationHistoryManager) ResetConversation() {
	chm.conversationHistory = []models.Message{}
	chm.nextIndex = 0
	utils.PrintSuccess("Conversation history reset")
}

func (chm *ConversationHistoryManager) GetConversationHistory() []models.Message {
	return chm.conversationHistory
}

func (chm *ConversationHistoryManager) SetConversationHistory(history []models.Message) {
	chm.conversationHistory = history
}

func (chm *ConversationHistoryManager) GetConversationStats() map[string]int {
	return map[string]int{
		"total_messages": len(chm.conversationHistory),
		"user_messages":  chm.countMessagesByRole(models.MessageRoleUser),
		"bot_messages":   chm.countMessagesByRole(models.MessageRoleAssistant),
	}
}

func (chm *ConversationHistoryManager) countMessagesByRole(role models.MessageRole) int {
	count := 0
	for _, msg := range chm.conversationHistory {
		if msg.Role == models.MessageRole(role) {
			count++
		}
	}
	return count
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
