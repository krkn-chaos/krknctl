package providers

type ScenarioProvider interface {
	GetScenarios() (*[]ScenarioTag, error)
}
