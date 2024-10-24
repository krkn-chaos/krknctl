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
		Version:              "0.0.1",
		QuayProtocol:         "https",
		QuayHost:             "quay.io",
		QuayOrg:              "redhat-chaos",
		QuayRegistry:         "krkn-hub",
		QuayRepositoryApi:    "api/v1/repository",
		ContainerPrefix:      "krknctl-containers",
		KubeconfigPrefix:     "krknctl-kubeconfig",
		DarwinSocketTemplate: "unix://%s/.local/share/containers/podman/machine/podman.sock",
		LinuxSocketTemplate:  "unix://run/user/%s/podman/podman.sock",
		LinuxSocketRoot:      "unix://run/podman/podman.sock",
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
	cm := ContainerManager{
		Config: conf,
	}
	quayProvider := quay.ScenarioProvider{}
	scenario, err := quayProvider.GetScenarioDetail("cpu-hog", conf.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	kubeconfig, err := container_manager.PrepareKubeconfig(nil, getTestConfig())
	assert.Nil(t, err)
	socket, err := cm.GetContainerRuntimeSocket()
	assert.Nil(t, err)
	containerId, err := cm.RunAndStream(conf.GetQuayImageUri()+":"+scenario.Name,
		scenario.Name,
		*socket,
		env, false, map[string]string{}, *kubeconfig, scenario.KubeconfigPath)
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

}
