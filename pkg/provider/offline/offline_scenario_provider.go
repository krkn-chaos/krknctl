package offline

import (
	"errors"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

type ScenarioProvider struct {
	Config *config.Config
}

func (p *ScenarioProvider) GetScenarios() (*[]models.ScenarioTag, error) {
	return nil, errors.New("not yet implemented")
}

func (p *ScenarioProvider) GetScenarioDetail(scenario string) (*models.ScenarioDetail, error) {
	return nil, errors.New("not yet implemented")
}
