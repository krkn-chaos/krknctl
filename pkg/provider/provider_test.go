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

func TestPopulateBooleanLabels_PrivilegedTrue(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
			`LABEL krknctl.privileged="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.Privileged)
}

func TestPopulateBooleanLabels_PrivilegedFalse(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
			`LABEL krknctl.privileged="false"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.False(t, detail.Privileged)
}

func TestPopulateBooleanLabels_PrivilegedMissing_DefaultsFalse(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.is_a_scenario="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.False(t, detail.Privileged)
}

func TestPopulateBooleanLabels_PrivilegedInvalidBool(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.privileged="notabool"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid boolean value")
}

func TestPopulateBooleanLabels_PrivilegedGlobalEnvironment_Skipped(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.privileged="true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, true)
	assert.Nil(t, err)
	assert.False(t, detail.Privileged)
}

func TestPopulateBooleanLabels_PrivilegedWhitespaceAroundEquals(t *testing.T) {
	p := getTestProvider(t)
	detail := &models.ScenarioDetail{}
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			`LABEL krknctl.privileged = "true"`,
		}},
	}

	err := p.PopulateBooleanLabels(detail, layers, false)
	assert.Nil(t, err)
	assert.True(t, detail.Privileged)
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

func TestGetKrknctlLabel_LabelInsideValue_DoesNotMatch(t *testing.T) {
	// The target label appears only inside another label's quoted value; must not match.
	layers := []ContainerLayer{
		mockLayer{commands: []string{`LABEL note="LABEL krknctl.privileged=true"`}},
	}
	result := GetKrknctlLabel("krknctl.privileged=", layers)
	assert.Nil(t, result)
}

func TestGetKrknctlLabel_LabelInsideValue_WithLeadingSpace_DoesNotMatch(t *testing.T) {
	// Same scenario with leading whitespace before the outer LABEL command.
	layers := []ContainerLayer{
		mockLayer{commands: []string{`  LABEL note="LABEL krknctl.is_a_scenario=true"`}},
	}
	result := GetKrknctlLabel("krknctl.is_a_scenario=", layers)
	assert.Nil(t, result)
}

func TestGetKrknctlLabel_ActualLabel_WithLeadingSpace_Matches(t *testing.T) {
	// Leading whitespace on the command itself is fine; the label is real.
	layers := []ContainerLayer{
		mockLayer{commands: []string{`  LABEL krknctl.privileged="true"`}},
	}
	result := GetKrknctlLabel("krknctl.privileged=", layers)
	assert.NotNil(t, result)
}

func TestGetKrknctlLabel_DockerHistoryFormat_Matches(t *testing.T) {
	// Docker layer history stores commands as /bin/sh -c #(nop) LABEL ...
	// GetKrknctlLabel must match the LABEL token even when it is not at position 0.
	layers := []ContainerLayer{
		mockLayer{commands: []string{
			"/bin/sh",
			"-c",
			`#(nop)  LABEL krknctl.privileged="true"`,
		}},
	}
	result := GetKrknctlLabel("krknctl.privileged=", layers)
	assert.NotNil(t, result)
}

func TestGetKrknctlLabel_ShellCNopFormat_Matches(t *testing.T) {
	// Single-string Quay / registry format: the whole command is one string.
	layers := []ContainerLayer{
		mockLayer{commands: []string{`/bin/sh -c #(nop)  LABEL krknctl.title="My Scenario"`}},
	}
	result := GetKrknctlLabel("krknctl.title=", layers)
	assert.NotNil(t, result)
}