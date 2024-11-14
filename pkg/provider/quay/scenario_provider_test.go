package quay

import (
	"encoding/json"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func getTestConfig() krknctlconfig.Config {
	data := `
{
  "version": "0.0.1",
  "quay_protocol": "https",
  "quay_host": "quay.io",
  "quay_org": "krkn-chaos",
  "quay_registry": "krknctl-test",
  "quay_repositoryApi": "api/v1/repository",
  "container_prefix": "krknctl",
  "kubeconfig_prefix": "krknctl-kubeconfig",
  "krknctl_logs": "krknct-log",
  "podman_darwin_socket_template": "unix://%s/.local/share/containers/podman/machine/podman.sock",
  "podman_linux_socket_template": "unix://run/user/%d/podman/podman.sock",
  "podman_socket_root_linux": "unix://run/podman/podman.sock",
  "docker_socket_root": "unix:///var/run/docker.sock",
  "default_container_platform": "Podman",
  "metrics_profile_path" : "/home/krkn/kraken/config/metrics-aggregated.yaml",
  "alerts_profile_path":"/home/krkn/kraken/config/alerts",
  "kubeconfig_path": "/home/krkn/.kube/config",
  "label_title": "krknctl.title",
  "label_description": "krknctl.description",
  "label_input_fields": "krknctl.input_fields",
  "label_title_regex": "LABEL krknctl\\.title=\\\"?(.*)\\\"?",
  "label_description_regex": "LABEL krknctl\\.description=\\\"?(.*)\\\"?",
  "label_input_fields_regex": "LABEL krknctl\\.input_fields=\\'?(\\[.*\\])\\'?"
}

`
	conf := krknctlconfig.Config{}
	_ = json.Unmarshal([]byte(data), &conf)

	_ = krknctlconfig.Config{
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "krkn-chaos",
		QuayRegistry:      "krknctl-test",
		QuayRepositoryApi: "api/v1/repository",
	}
	return conf
}

func getWrongConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "krkn-chaos",
		QuayRegistry:      "do_not_exist",
		QuayRepositoryApi: "api/v1/repository",
	}
}

func TestQuayScenarioProvider_GetScenarios(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{Config: &config}

	scenarios, err := provider.GetScenarios(config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.Greater(t, len(*scenarios), 0)
	assert.NotEqual(t, (*scenarios)[0].Name, "")
	assert.NotEqual(t, (*scenarios)[0].Digest, "")
	assert.NotEqual(t, (*scenarios)[0].Size, 0)
	assert.NotEqual(t, (*scenarios)[0].LastModified, time.Time{})
	assert.NotNil(t, scenarios)

	wrongConfig := getWrongConfig()
	wrongProvider := ScenarioProvider{Config: &wrongConfig}

	_, err = wrongProvider.GetScenarios(wrongConfig.GetQuayRepositoryApiUri())
	assert.NotNil(t, err)
}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig()
	provider := ScenarioProvider{Config: &config}

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

	scenario, err = provider.GetScenarioDetail("not-found", config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.Nil(t, scenario)

}

func TestScenarioProvider_ScaffoldScenarios(t *testing.T) {
	config, _ := krknctlconfig.LoadConfig()

	provider := ScenarioProvider{Config: &config}

	scenarios, err := provider.GetScenarios(config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	var scenarioNames []string

	for _, scenario := range *scenarios {
		scenarioNames = append(scenarioNames, scenario.Name)
	}

	json, err := provider.ScaffoldScenarios(scenarioNames, config.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, config.GetQuayRepositoryApiUri())
	assert.Nil(t, json)
	assert.NotNil(t, err)

}
