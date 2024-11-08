package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/offline"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
)

type ProviderFactory struct {
	Config *config.Config
}

func NewProviderFactory(config *config.Config) *ProviderFactory {
	return &ProviderFactory{Config: config}
}

func (p *ProviderFactory) NewInstance(mode provider.Mode) provider.ScenarioDataProvider {
	switch mode {
	case provider.Online:
		return &quay.ScenarioProvider{Config: p.Config}
	case provider.Offline:
		return &offline.ScenarioProvider{}
	}
	return nil
}
