package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/provider/offline"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
)

type ScenarioDataProvider interface {
	GetScenarios(dataSource string) (*[]models.ScenarioTag, error)
	GetScenarioDetail(scenario string, dataSource string) (*models.ScenarioDetail, error)
}

type Mode int64

const (
	Online = iota
	Offline
)

type ProviderFactory struct {
	Config *config.Config
}

func NewProviderFactory(config *config.Config) *ProviderFactory {
	return &ProviderFactory{Config: config}
}

func (p *ProviderFactory) NewInstance(mode Mode) ScenarioDataProvider {
	switch mode {
	case Online:
		return &quay.ScenarioProvider{}
	case Offline:
		return &offline.ScenarioProvider{}
	default:
		return nil
	}
}
