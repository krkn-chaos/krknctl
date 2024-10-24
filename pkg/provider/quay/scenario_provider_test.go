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
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "rh_ee_tsebasti",
		QuayRegistry:      "krknctl",
		QuayRepositoryApi: "api/v1/repository",
	}
}

func getWrongConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "rh_ee_tsebasti",
		QuayRegistry:      "do_not_exist",
		QuayRepositoryApi: "api/v1/repository",
	}
}

func TestQuayScenarioProvider_GetScenarios(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{}

	scenarios, err := provider.GetScenarios(config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.Greater(t, len(*scenarios), 0)
	assert.NotEqual(t, (*scenarios)[0].Name, "")
	assert.NotEqual(t, (*scenarios)[0].Digest, "")
	assert.NotEqual(t, (*scenarios)[0].Size, 0)
	assert.NotEqual(t, (*scenarios)[0].LastModified, time.Time{})
	assert.NotNil(t, scenarios)

	wrongConfig := getWrongConfig()
	wrongProvider := ScenarioProvider{}

	_, err = wrongProvider.GetScenarios(wrongConfig.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{}

	scenario, err := provider.GetScenarioDetail("cpu-hog", config.GetQuayRepositoryApiUri())

	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Equal(t, len(scenario.Fields), 5)

	scenario, err = provider.GetScenarioDetail("cpu-memory-notitle", config.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.title LABEL not found in tag: cpu-memory-notitle"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nodescription", config.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.description LABEL not found in tag: cpu-memory-nodescription"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-noinput", config.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.input_fields LABEL not found in tag: cpu-memory-noinput"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nokubeconfig", config.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.kubeconfig_path LABEL not found in tag: cpu-memory-nokubeconfig"))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("not-found", config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.Nil(t, scenario)

}
