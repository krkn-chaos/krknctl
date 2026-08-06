package provider

import (
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/assert"
)

type mockLayer struct {
	commands []string
}

func (m mockLayer) GetCommands() []string {
	return m.commands
}

func getTestProvider(t *testing.T) BaseScenarioProvider {
	cfg, err := config.LoadConfig()
	assert.Nil(t, err)
	return BaseScenarioProvider{
		Config: cfg,
		Cache:  cache.NewCache(),
	}
}

func TestPopulateBooleanLabels_BothTrue(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
			`LABEL krknctl.has_rollback="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.IsAScenario)
	assert.True(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_BothFalse(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="false"`,
			`LABEL krknctl.has_rollback="false"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.False(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_MissingLabels(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.title="some title"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.False(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_GlobalEnvironment_Skipped(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
			`LABEL krknctl.has_rollback="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, true)
	assert.Nil(t, err)
	assert.False(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_OnlyIsAScenario(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_InvalidBoolValue(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="notabool"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid boolean value")
}

func TestPopulateBooleanLabels_LabelsAcrossMultipleLayers(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL krknctl.is_a_scenario="true"`}},
		mockLayer{commands: []string{`LABEL krknctl.has_rollback="true"`}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.IsAScenario)
	assert.True(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_EmptyLayers(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	var layers []ContainerLayer

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.False(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestPopulateBooleanLabels_NilDetail(t *testing.T) {
	p := getTestProvider(t)
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL krknctl.is_a_scenario="true"`}},
	}

	err := p.PopulateBooleanLabels(nil, layers, false)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "scenario detail cannot be nil")
}

func TestPopulateBooleanLabels_WhitespaceAroundEquals(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario = "true"`,
			`LABEL krknctl.has_rollback = "false"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.IsAScenario)
	assert.False(t, detail.HasRollback)
}

func TestGetKrknctlLabel_WhitespaceAroundEquals(t *testing.T) {
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL krknctl.is_a_scenario = "true"`}},
	}
	result := GetKrknctlLabel("krknctl.is_a_scenario=", layers)
	assert.NotNil(t, result)
	assert.Contains(t, *result, "krknctl.is_a_scenario")
}

func TestGetKrknctlLabel_NoTrailingEquals(t *testing.T) {
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL krknctl.title="My Title"`}},
	}
	result := GetKrknctlLabel("krknctl.title=", layers)
	assert.NotNil(t, result)
	assert.Contains(t, *result, "My Title")
}

func TestGetKrknctlLabel_NotFound(t *testing.T) {
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL krknctl.title="My Title"`}},
	}
	result := GetKrknctlLabel("krknctl.is_a_scenario=", layers)
	assert.Nil(t, result)
}