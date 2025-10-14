package managers

import (
	"ai-agent/utils"
	"ai-agent/work-flows/agents"
	"ai-agent/work-flows/client"
	"ai-agent/work-flows/models"
	"fmt"
)

type PersonalizeManager struct {
	name   string
	client client.Client
	agents map[string]models.Agent
}

func NewPersonalizeManager(client client.Client) *PersonalizeManager {
	manager := &PersonalizeManager{
		name:   "PersonalizeManager",
		client: client,
		agents: make(map[string]models.Agent),
	}

	manager.RegisterAgents()
	return manager
}

func (pm *PersonalizeManager) RegisterAgents() {
	personalizeLessonAgent := agents.NewPersonalizeLessonAgent(pm.client)
	pm.agents[personalizeLessonAgent.Name()] = personalizeLessonAgent

	utils.PrintSuccess("PersonalizeManager initialized with agents:")
	for _, agent := range pm.agents {
		utils.PrintInfo(fmt.Sprintf("- %s: %s", agent.Name(), agent.GetDescription()))
	}
}

func (pm *PersonalizeManager) Name() string {
	return pm.name
}

func (pm *PersonalizeManager) GetDescription() string {
	return "Manages and coordinates personalize-related agents for lesson detail creation"
}

func (pm *PersonalizeManager) ProcessTask(task models.JobRequest) *models.JobResponse {
	utils.PrintInfo(fmt.Sprintf("PersonalizeManager processing task: %s", task.Task))

	agent, err := pm.SelectAgent(task)
	if err != nil {
		utils.PrintError(fmt.Sprintf("Agent selection failed: %s", err.Error()))
		return &models.JobResponse{
			AgentName: pm.Name(),
			Success:   false,
			Result:    "",
			Error:     err.Error(),
		}
	}

	utils.PrintInfo(fmt.Sprintf("Delegating to agent: %s", agent.Name()))
	return agent.ProcessTask(task)
}

func (pm *PersonalizeManager) SelectAgent(task models.JobRequest) (models.Agent, error) {
	for _, agent := range pm.agents {
		if agent.CanHandle(task.Task) {
			utils.PrintInfo(fmt.Sprintf("Selected agent: %s for task: %s", agent.Name(), task.Task))
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no suitable agent found for task: %s", task.Task)
}

func (pm *PersonalizeManager) GetAgent(name string) (models.Agent, bool) {
	agent, exists := pm.agents[name]
	return agent, exists
}
