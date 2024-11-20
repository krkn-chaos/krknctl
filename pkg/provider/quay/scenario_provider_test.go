package quay

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
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
	conf.QuayRegistry = "krknctl-test"
	return conf
}

func getWrongConfig(t *testing.T) krknctlconfig.Config {
	conf := getConfig(t)
	conf.QuayRegistry = "do_not_exist"
	return conf
}

func TestScenarioProvider_GetRegistryImages(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{Config: &config}

	uri, err := config.GetQuayRepositoryApiUri()
	assert.NoError(t, err)
	scenarios, err := provider.GetRegistryImages(uri)
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
	wrongProvider := ScenarioProvider{Config: &wrongConfig}
	uri, err = wrongConfig.GetQuayRepositoryApiUri()
	assert.NoError(t, err)
	_, err = wrongProvider.GetRegistryImages(uri)
	assert.Error(t, err)

}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{Config: &config}
	uri, err := config.GetQuayRepositoryApiUri()
	assert.NoError(t, err)
	scenario, err := provider.GetScenarioDetail("cpu-hog", uri)

	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Equal(t, len(scenario.Fields), 5)

	scenario, err = provider.GetScenarioDetail("cpu-memory-notitle", uri)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.title LABEL not found in tag: cpu-memory-notitle"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nodescription", uri)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.description LABEL not found in tag: cpu-memory-nodescription"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-noinput", uri)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.input_fields LABEL not found in tag: cpu-memory-noinput"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("not-found", uri)
	assert.Nil(t, err)
	assert.Nil(t, scenario)

}

func TestQuayScenarioProvider_ScaffoldScenarios(t *testing.T) {
	config := getConfig(t)
	uri, err := config.GetQuayRepositoryApiUri()
	assert.NoError(t, err)
	provider := ScenarioProvider{Config: &config}

	scenarios, err := provider.GetRegistryImages(uri)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	var scenarioNames []string

	for _, scenario := range *scenarios {
		scenarioNames = append(scenarioNames, scenario.Name)
	}

	json, err := provider.ScaffoldScenarios(scenarioNames, uri)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, uri)
	assert.Nil(t, json)
	assert.NotNil(t, err)

}
