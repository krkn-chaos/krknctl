package providers

import (
	"errors"
	"github.com/krkn-chaos/krknctl/internal/config"
)

type OfflineScenarioProvider struct {
	config *config.Config
}

func (p *OfflineScenarioProvider) GetScenarios() (*[]ScenarioTag, error) {
	return nil, errors.New("not yet implemented")
}
