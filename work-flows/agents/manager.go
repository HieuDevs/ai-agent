package agents

import (
	"fmt"
	"strings"

	"ai-agent/utils"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"

	"github.com/fatih/color"
)

type AgentManager struct {
	apiClient  client.Client
	agents     map[string]models.Agent
	currentJob *models.JobRequest
}

func NewManager(apiKey string, level models.ConversationLevel, topic string) *AgentManager {
	client := client.NewOpenRouterClient(apiKey)

	manager := &AgentManager{
		apiClient: client,
		agents:    make(map[string]models.Agent),
	}

	manager.RegisterAgents(level, topic)
	return manager
}

func (m *AgentManager) RegisterAgents(level models.ConversationLevel, topic string) {
	conversationAgent := NewConversationAgent(m.apiClient, level, topic)

	m.agents[conversationAgent.Name()] = conversationAgent

	utils.PrintSuccess("Agent Manager initialized with agents:")
	for _, agent := range m.agents {
		cyan := color.New(color.FgCyan)
		cyan.Printf("- %s: %s\n", agent.Name(), agent.GetDescription())
	}
}

func (m *AgentManager) SelectAgent(task models.JobRequest) (models.Agent, error) {
	for _, agent := range m.agents {
		if agent.CanHandle(task.Task) {
			utils.PrintInfo(fmt.Sprintf("Selected agent: %s for task: %s", agent.Name(), task.Task))
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no suitable agent found for task: %s", task.Task)
}

func (m *AgentManager) ListAgents() {
	utils.PrintInfo("Available Agents:")
	for _, agent := range m.agents {
		cyan := color.New(color.FgCyan)
		yellow := color.New(color.FgYellow)
		cyan.Printf("• %s\n", agent.Name())
		yellow.Printf("  Description: %s\n", agent.GetDescription())
		yellow.Printf("  Capabilities: %s\n", strings.Join(agent.Capabilities(), ", "))
	}
}

func (m *AgentManager) GetAgent(name string) (models.Agent, bool) {
	agent, exists := m.agents[name]
	return agent, exists
}

func (m *AgentManager) ProcessJob(job models.JobRequest) *models.JobResponse {
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

	utils.PrintInfo(fmt.Sprintf("Processing job with agent: %s", agent.Name()))
	return agent.ProcessTask(job)
}
