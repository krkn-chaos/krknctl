package provider

import "github.com/krkn-chaos/krknctl/pkg/provider/models"

type Mode int64

const (
	Online = iota
	Offline
)

type ScenarioDataProvider interface {
	GetRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error)
	GetGlobalEnvironment(registry *models.RegistryV2) (*models.ScenarioDetail, error)
	GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error)
	ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2) (*string, error)
}
