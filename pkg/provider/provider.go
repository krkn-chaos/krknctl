package provider

import "github.com/krkn-chaos/krknctl/pkg/provider/models"

type Mode int64

const (
	Online = iota
	Offline
)

type ScenarioDataProvider interface {
	GetRegistryImages() (*[]models.ScenarioTag, error)
	GetGlobalEnvironment() (*models.ScenarioDetail, error)
	GetScenarioDetail(scenario string) (*models.ScenarioDetail, error)
	ScaffoldScenarios(scenarios []string, includeGlobalEnv bool) (*string, error)
}
