package providers

import "github.com/krkn-chaos/krknctl/internal/config"

type Mode int64

const (
	Online = iota
	Offline
)

type ProviderFactory struct {
	config *config.Config
}

func NewProviderFactory(config *config.Config) *ProviderFactory {
	return &ProviderFactory{config: config}
}

func (p *ProviderFactory) NewInstance(mode Mode) ScenarioProvider {
	switch mode {
	case Online:

		return &QuayScenarioProvider{config: p.config}
	case Offline:
		return &OfflineScenarioProvider{config: p.config}
	default:
		return nil
	}
}
