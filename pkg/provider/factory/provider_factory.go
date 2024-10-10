package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/provider/offline"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
)

type ScenarioProvider interface {
	GetScenarios() (*[]models.ScenarioTag, error)
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

func (p *ProviderFactory) NewInstance(mode Mode) ScenarioProvider {
	switch mode {
	case Online:
		return &quay.ScenarioProvider{Config: p.Config}
	case Offline:
		return &offline.ScenarioProvider{Config: p.Config}
	default:
		return nil
	}
}
