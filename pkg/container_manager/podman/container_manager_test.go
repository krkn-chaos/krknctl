package podman

import (
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/stretchr/testify/assert"
	"os"
	"os/user"
	"strconv"
	"testing"
)

func getTestConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:                    "0.0.1",
		QuayProtocol:               "https",
		QuayHost:                   "quay.io",
		QuayOrg:                    "krkn-chaos",
		QuayRegistry:               "krkn-hub",
		QuayRepositoryApi:          "api/v1/repository",
		ContainerPrefix:            "krknctl-containers",
		KubeconfigPrefix:           "krknctl-kubeconfig",
		PodmanDarwinSocketTemplate: "unix://%s/.local/share/containers/podman/machine/podman.sock",
		PodmanLinuxSocketTemplate:  "unix://run/user/%d/podman/podman.sock",
		PodmanSocketRoot:           "unix://run/podman/podman.sock",
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
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{}
	scenario, err := quayProvider.GetScenarioDetail("node-cpu-hog", conf.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	kubeconfig, err := container_manager.PrepareKubeconfig(nil, getTestConfig())
	assert.Nil(t, err)
	assert.NotNil(t, kubeconfig)
	fmt.Println("KUBECONFIG PARSED -> " + *kubeconfig)

	envuid := os.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}
	socket, err := cm.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)

	fmt.Println("CONTAINER SOCKET -> " + *socket)
	containerId, err := cm.RunAttached(conf.GetQuayImageUri()+":"+scenario.Name, scenario.Name, *socket, env, false, map[string]string{})
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

}
