package models

type Agent interface {
	Name() string
	GetDescription() string
	Capabilities() []string
	CanHandle(task string) bool
	ProcessTask(task JobRequest) *JobResponse
}
