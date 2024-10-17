package offline

import (
	"errors"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

type ScenarioProvider struct {
}

func (p *ScenarioProvider) GetScenarios(dataSource string) (*[]models.ScenarioTag, error) {
	return nil, errors.New("not yet implemented")
}

func (p *ScenarioProvider) GetScenarioDetail(scenario string, dataSource string) (*models.ScenarioDetail, error) {
	return nil, errors.New("not yet implemented")
}
