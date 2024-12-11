package offline

import (
	"errors"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

type ScenarioProvider struct {
}

func (p *ScenarioProvider) GetRegistryImages() (*[]models.ScenarioTag, error) {
	return nil, errors.New("not yet implemented")
}

func (p *ScenarioProvider) GetScenarioDetail(scenario string) (*models.ScenarioDetail, error) {
	return nil, errors.New("not yet implemented")
}

func (p *ScenarioProvider) ScaffoldScenarios(scenarios []string) (*string, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ScenarioProvider) GetGlobalEnvironment() (*models.ScenarioDetail, error) {
	//TODO implement me
	panic("implement me")
}
