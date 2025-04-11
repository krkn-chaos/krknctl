package factory

import (
	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/krkn-chaos/krknctl/pkg/provider/registryv2"
)

type ProviderFactory struct {
	Config *config.Config
}

func NewProviderFactory(config *config.Config) *ProviderFactory {
	return &ProviderFactory{Config: config}
}

func (p *ProviderFactory) NewInstance(mode provider.Mode) provider.ScenarioDataProvider {
	switch mode {
	case provider.Quay:
		return &quay.ScenarioProvider{BaseScenarioProvider: provider.BaseScenarioProvider{Config: *p.Config, Cache: cache.NewCache()}}
	case provider.Private:
		return &registryv2.ScenarioProvider{BaseScenarioProvider: provider.BaseScenarioProvider{Config: *p.Config, Cache: cache.NewCache()}}
	}
	return nil
}
