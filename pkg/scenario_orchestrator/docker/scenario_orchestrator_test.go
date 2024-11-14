package docker

import (
	"encoding/json"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"os/user"
	"strconv"
	"testing"
)

func getTestConfig() krknctlconfig.Config {
	_ = krknctlconfig.Config{
		Version:                    "0.0.1",
		QuayProtocol:               "https",
		QuayHost:                   "quay.io",
		QuayOrg:                    "krkn-chaos",
		QuayRegistry:               "krkn-hub",
		QuayRepositoryApi:          "api/v1/repository",
		ContainerPrefix:            "krknctl-containers",
		KubeconfigPrefix:           "krknctl-kubeconfig",
		PodmanDarwinSocketTemplate: "unix://%s/.local/share/containers/podman/machine/podman.sock",
		PodmanLinuxSocketTemplate:  "unix:///run/user/%d/podman/podman.sock",
		PodmanSocketRoot:           "unix:///run/podman/podman.sock",
		DockerSocketRoot:           "unix:///var/run/docker.sock",
	}

	data := `
{
  "version": "0.0.1",
  "quay_protocol": "https",
  "quay_host": "quay.io",
  "quay_org": "krkn-chaos",
  "quay_registry": "krkn-hub",
  "quay_repositoryApi": "api/v1/repository",
  "container_prefix": "krknctl",
  "kubeconfig_prefix": "krknctl-kubeconfig",
  "krknctl_logs": "krknct-log",
  "podman_darwin_socket_template": "unix://%s/.local/share/containers/podman/machine/podman.sock",
  "podman_linux_socket_template": "unix://run/user/%d/podman/podman.sock",
  "podman_socket_root_linux": "unix://run/podman/podman.sock",
  "podman_running_state": "running",
  "docker_socket_root": "unix:///var/run/docker.sock",
  "docker_running_state": "running",
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
		Version:                    "0.0.1",
		QuayProtocol:               "https",
		QuayHost:                   "quay.io",
		QuayOrg:                    "krkn-chaos",
		QuayRegistry:               "krknctl-test",
		QuayRepositoryApi:          "api/v1/repository",
		ContainerPrefix:            "krknctl",
		KubeconfigPrefix:           "krknctl-kubeconfig",
		PodmanDarwinSocketTemplate: "unix://%s/.local/share/containers/podman/machine/podman.sock",
		PodmanLinuxSocketTemplate:  "unix://run/user/%d/podman/podman.sock",
		PodmanSocketRoot:           "unix://run/podman/podman.sock",
	}

	return conf

}

func TestscenarioOrchestrator_Run(t *testing.T) {
	env := map[string]string{
		"CHAOS_DURATION": "2",
		"CORES":          "1",
		"CPU_PERCENTAGE": "60",
		"NAMESPACE":      "default",
	}
	conf := getTestConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &conf}
	scenario, err := quayProvider.GetScenarioDetail("node-cpu-hog", conf.GetQuayRepositoryApiUri())
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	kubeconfig, err := utils.PrepareKubeconfig(nil, getTestConfig())
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
	containerId, err := cm.RunAttached(conf.GetQuayImageUri()+":"+scenario.Name, scenario.Name, *socket, env, false, map[string]string{}, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)
}

func TestScenarioOrchestrator_ListRunningContainers(t *testing.T) {
	conf := getTestConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
	kubeconfig, err := utils.PrepareKubeconfig(nil, conf)
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

	containers, err := cm.ListRunningContainers(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, containers)
}
