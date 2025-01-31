package quay

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	provider2 "github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func getTestConfig(t *testing.T) krknctlconfig.Config {
	conf := getConfig(t)
	conf.QuayScenarioRegistry = "krknctl-test"
	return conf
}

func getWrongConfig(t *testing.T) krknctlconfig.Config {
	conf := getConfig(t)
	conf.QuayScenarioRegistry = "do_not_exist"
	return conf
}

func TestScenarioProvider_GetRegistryImages(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{
		provider2.BaseScenarioProvider{
			Config: config,
		},
	}
	scenarios, err := provider.GetRegistryImages(nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	assert.Greater(t, len(*scenarios), 0)
	for i, _ := range *scenarios {
		assert.NotEqual(t, (*scenarios)[i].Name, "")
		assert.NotEqual(t, (*scenarios)[i].Digest, "")
		assert.NotEqual(t, (*scenarios)[i].Size, 0)
		assert.NotEqual(t, (*scenarios)[i].LastModified, time.Time{})
	}

	wrongConfig := getWrongConfig(t)
	wrongProvider := ScenarioProvider{
		provider2.BaseScenarioProvider{
			Config: wrongConfig,
		},
	}
	_, err = wrongProvider.GetRegistryImages(nil)
	assert.Error(t, err)

}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{
		provider2.BaseScenarioProvider{
			Config: config,
		},
	}
	scenario, err := provider.GetScenarioDetail("cpu-hog", nil)

	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Equal(t, len(scenario.Fields), 5)

	scenario, err = provider.GetScenarioDetail("cpu-memory-notitle", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.title LABEL not found in tag: cpu-memory-notitle"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nodescription", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.description LABEL not found in tag: cpu-memory-nodescription"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-noinput", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.input_fields LABEL not found in tag: cpu-memory-noinput"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("not-found", nil)
	assert.Nil(t, err)
	assert.Nil(t, scenario)

}

func TestQuayScenarioProvider_ScaffoldScenarios(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{
		provider2.BaseScenarioProvider{
			Config: config,
		},
	}

	scenarios, err := provider.GetRegistryImages(nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	scenarioNames := []string{"node-cpu-hog", "node-memory-hog", "dummy-scenario"}

	json, err := provider.ScaffoldScenarios(scenarioNames, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios(scenarioNames, true, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, false, nil)
	assert.Nil(t, json)
	assert.NotNil(t, err)

}

func TestQuayScenarioProvider_GetGlobalEnvironment(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{
		provider2.BaseScenarioProvider{
			Config: config,
		},
	}
	config.QuayBaseImageRegistry = "krknctl-test"
	baseImageScenario, err := provider.GetGlobalEnvironment(nil)
	assert.Nil(t, err)
	assert.NotNil(t, baseImageScenario)
	assert.Greater(t, len(baseImageScenario.Fields), 0)
}
