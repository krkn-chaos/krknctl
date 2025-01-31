package factory

import (
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
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
		return &quay.ScenarioProvider{BaseScenarioProvider: provider.BaseScenarioProvider{Config: *p.Config}}
	case provider.Offline:
		panic("unimplemented")
	}
	return nil
}
