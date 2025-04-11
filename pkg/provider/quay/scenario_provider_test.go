package quay

import (
	jsonparser "encoding/json"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	providerinterface "github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/assert"
	"os"
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
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
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
		providerinterface.BaseScenarioProvider{
			Config: wrongConfig,
			Cache:  cache.NewCache(),
		},
	}
	_, err = wrongProvider.GetRegistryImages(nil)
	assert.Error(t, err)

}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
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
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	scenarios, err := provider.GetRegistryImages(nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	scenarioNames := []string{"node-cpu-hog", "node-memory-hog", "dummy-scenario"}

	json, err := provider.ScaffoldScenarios(scenarioNames, false, nil, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	fmt.Println(os.Getwd())
	seed := providerinterface.ScaffoldSeed{
		Path:              "../../../tests/data/scaffold-seed.json",
		NumberOfScenarios: 1000,
	}

	json, err = provider.ScaffoldScenarios([]string{}, false, nil, false, &seed)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	var scenariodetails map[string]models.ScenarioDetail
	err = jsonparser.Unmarshal([]byte(*json), &scenariodetails)
	assert.Nil(t, err)
	assert.Equal(t, len(scenariodetails), seed.NumberOfScenarios)

	json, err = provider.ScaffoldScenarios(scenarioNames, false, nil, true, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	var parsedScenarios map[string]map[string]interface{}
	err = jsonparser.Unmarshal([]byte(*json), &parsedScenarios)
	assert.Nil(t, err)
	for el := range parsedScenarios {
		assert.NotEqual(t, el, "comment")
		_, ok := parsedScenarios[el]["depends_on"]
		assert.False(t, ok)
	}

	json, err = provider.ScaffoldScenarios(scenarioNames, true, nil, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, false, nil, false, nil)
	assert.Nil(t, json)
	assert.NotNil(t, err)

}

func TestQuayScenarioProvider_GetGlobalEnvironment(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	config.QuayBaseImageRegistry = "krknctl-test"
	baseImageScenario, err := provider.GetGlobalEnvironment(nil, "node-cpu-hog")
	assert.Nil(t, err)
	assert.NotNil(t, baseImageScenario)
	assert.Greater(t, len(baseImageScenario.Fields), 0)
}
