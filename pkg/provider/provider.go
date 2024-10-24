package provider

import "github.com/krkn-chaos/krknctl/pkg/provider/models"

type Mode int64

const (
	Online = iota
	Offline
)

type ScenarioDataProvider interface {
	GetScenarios(dataSource string) (*[]models.ScenarioTag, error)
	GetScenarioDetail(scenario string, dataSource string) (*models.ScenarioDetail, error)
}
