package quay

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func getTestConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		QuayRegistry: "rh_ee_tsebasti/krknctl",
		QuayApi:      "https://quay.io/api/v1/repository",
		Version:      "0.0.1",
	}
}

func getWrongConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		QuayRegistry: "rh_ee_tsebasti/do_not_exist",
		QuayApi:      "https://quay.io/api/v1/repository",
		Version:      "0.0.1",
	}
}

func TestQuayScenarioProvider_GetScenarios(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{
		Config: &config,
	}

	scenarios, err := provider.GetScenarios()
	assert.Nil(t, err)
	assert.Len(t, *scenarios, 1)
	assert.NotEqual(t, (*scenarios)[0].Name, "")
	assert.NotEqual(t, (*scenarios)[0].Digest, "")
	assert.NotEqual(t, (*scenarios)[0].Size, 0)
	assert.NotEqual(t, (*scenarios)[0].LastModified, time.Time{})
	assert.NotNil(t, scenarios)

	wrongConfig := getWrongConfig()
	wrongProvider := ScenarioProvider{
		Config: &wrongConfig,
	}

	_, err = wrongProvider.GetScenarios()
	assert.NotNil(t, err)
}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{
		Config: &config,
	}

	scenario, err := provider.GetScenarioDetail("cpu-hog")

	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Equal(t, len(scenario.Fields), 5)

	scenario, err = provider.GetScenarioDetail("cpu-memory-notitle")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krkn.title LABEL not found in tag: cpu-memory-notitle"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nodescription")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krkn.description LABEL not found in tag: cpu-memory-nodescription"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-noinput")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krkn.inputfields LABEL not found in tag: cpu-memory-noinput"))
	assert.Nil(t, scenario)

}
