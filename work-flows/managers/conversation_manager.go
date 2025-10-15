package managers

import (
	"fmt"
	"strings"

	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"ai-agent/work-flows/services"

	"github.com/fatih/color"
)

type ConversationManager struct {
	apiClient      client.Client
	agents         map[string]models.Agent
	currentJob     *models.JobRequest
	sessionId      string
	historyManager *services.ConversationHistoryManager
}

func NewConversationManager(apiKey string, level models.ConversationLevel, topic string, language string, sessionId string) *ConversationManager {
	client := client.NewOpenRouterClient(apiKey)

	manager := &ConversationManager{
		apiClient:      client,
		agents:         make(map[string]models.Agent),
		sessionId:      sessionId,
		historyManager: services.NewConversationHistoryManager(),
	}

	manager.RegisterAgents(level, topic, language)
	return manager
}

func (m *ConversationManager) RegisterAgents(level models.ConversationLevel, topic string, language string) {
	conversationAgent := agents.NewConversationAgent(m.apiClient, level, topic, m.historyManager)
	// Get title from conversation agent
	title := conversationAgent.GetTitle()
	if title == "" {
		title = topic
	}
	suggestionAgent := agents.NewSuggestionAgent(m.apiClient, level, title, language)
	evaluateAgent := agents.NewEvaluateAgent(m.apiClient, level, title, language)
	assessmentAgent := agents.NewAssessmentAgent(m.apiClient, language)

	m.agents[conversationAgent.Name()] = conversationAgent
	m.agents[suggestionAgent.Name()] = suggestionAgent
	m.agents[evaluateAgent.Name()] = evaluateAgent
	m.agents[assessmentAgent.Name()] = assessmentAgent

	utils.PrintSuccess("Agent Manager initialized with agents:")
	for _, agent := range m.agents {
		cyan := color.New(color.FgCyan)
		cyan.Printf("- %s: %s\n", agent.Name(), agent.GetDescription())
	}
}

func (m *ConversationManager) SelectAgent(task models.JobRequest) (models.Agent, error) {
	for _, agent := range m.agents {
		if agent.CanHandle(task.Task) {
			utils.PrintInfo(fmt.Sprintf("Selected agent: %s for task: %s", agent.Name(), task.Task))
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no suitable agent found for task: %s", task.Task)
}

func (m *ConversationManager) ListAgents() {
	utils.PrintInfo("Available Agents:")
	for _, agent := range m.agents {
		cyan := color.New(color.FgCyan)
		yellow := color.New(color.FgYellow)
		cyan.Printf("• %s\n", agent.Name())
		yellow.Printf("  Description: %s\n", agent.GetDescription())
		yellow.Printf("  Capabilities: %s\n", strings.Join(agent.Capabilities(), ", "))
	}
}

func (m *ConversationManager) GetAgent(name string) (models.Agent, bool) {
	agent, exists := m.agents[name]
	return agent, exists
}

func (m *ConversationManager) GetHistoryManager() *services.ConversationHistoryManager {
	return m.historyManager
}

func (m *ConversationManager) GetConversationAgent() *agents.ConversationAgent {
	agent, exists := m.GetAgent("ConversationAgent")
	if !exists {
		return nil
	}
	return agent.(*agents.ConversationAgent)
}

func (m *ConversationManager) GetAssessmentAgent() *agents.AssessmentAgent {
	agent, exists := m.GetAgent("AssessmentAgent")
	if !exists {
		return nil
	}
	return agent.(*agents.AssessmentAgent)
}

func (m *ConversationManager) GetSessionId() string {
	return m.sessionId
}

func (m *ConversationManager) ProcessJob(job models.JobRequest) *models.JobResponse {
	m.currentJob = &job

	agent, err := m.SelectAgent(job)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Agent selection failed: %s", err.Error()))
		return &models.JobResponse{
			AgentName: "none",
			Success:   false,
			Result:    "",
			Error:     err.Error(),
		}
	}

	// Special handling for AssessmentAgent - it needs the history manager
	if agent.Name() == "AssessmentAgent" {
		job.Metadata = m.historyManager
	}

	utils.PrintInfo(fmt.Sprintf("Processing job with agent: %s", agent.Name()))
	return agent.ProcessTask(job)
}
