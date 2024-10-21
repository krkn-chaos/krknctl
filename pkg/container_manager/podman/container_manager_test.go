package podman

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/stretchr/testify/assert"
	"testing"
)

func getTestConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "rh_ee_tsebasti",
		QuayRegistry:      "krknctl",
		QuayRepositoryApi: "api/v1/repository",
		ContainerPrefix:   "krkn-",
	}
}

func TestConnect(t *testing.T) {
	env := map[string]string{
		"CHAOS_DURATION": "2",
		"CORES":          "1",
		"CPU_PERCENTAGE": "60",
		"NAMESPACE":      "default",
	}
	conf := getTestConfig()
	cm := ContainerManager{}
	quayProvider := quay.ScenarioProvider{}
	scenario, err := quayProvider.GetScenarioDetail("cpu-hog", conf.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	kubeconfig, err := container_manager.PrepareKubeconfig(nil)
	assert.Nil(t, err)

	containerId, err := cm.RunAndStream(conf.GetQuayImageUri()+":"+scenario.Name,
		scenario.Name,
		cm.GetContainerRuntimeUri(),
		env, false, map[string]string{}, *kubeconfig, scenario.KubeconfigPath)
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

}

func TestListPods(t *testing.T) {
	cm := ContainerManager{
		Config: getTestConfig(),
	}
	numberOfcontainers, err := cm.CleanContainers()
	assert.NotNil(t, numberOfcontainers)
	assert.Nil(t, err)
}
