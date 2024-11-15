package podman

import (
	"encoding/json"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"os/user"
	"strconv"
	"testing"
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
	return conf

}

func GetOriginalConfig() krknctlconfig.Config {
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
	return conf

}
func TestConnect(t *testing.T) {
	env := map[string]string{
		"CHAOS_DURATION": "2",
		"CORES":          "1",
		"CPU_PERCENTAGE": "60",
		"NAMESPACE":      "default",
	}
	conf := GetOriginalConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &conf}
	repositoryApiUri, err := conf.GetQuayRepositoryApiUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail(repositoryApiUri, "node-cpu-hog")
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
	imageUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)
	fmt.Println("CONTAINER SOCKET -> " + *socket)
	containerId, err := cm.RunAttached(imageUri+":"+scenario.Name, scenario.Name, *socket, env, false, map[string]string{}, os.Stdout, os.Stderr, nil)
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

}

func TestRunGraph(t *testing.T) {
	data := `
{
	"root":{
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	},
	"first-row-1":{
		"depends_on":"root",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	},
	"first-row-2":{
		"depends_on":"root",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	},
	"second-row":{
		"depends_on":"first-row-1",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	},
	"third-row-1":{
		"depends_on":"second-row",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	},
	"third-row-2":{
		"depends_on":"second-row",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"2"
		},
		"volumes":{}
	}
}
`
	conf := getTestConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &conf}
	repositoryApi, err := conf.GetQuayRepositoryApiUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", repositoryApi)
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

	nodes := make(map[string]models.ScenarioNode)
	err = json.Unmarshal([]byte(data), &nodes)
	assert.Nil(t, err)

	convertedNodes := make(map[string]dependencygraph.ParentProvider, len(nodes))

	// Populate the new map
	for key, node := range nodes {
		// Since ScenarioNode implements ParentProvider, this is valid
		convertedNodes[key] = node
	}

	graph, err := dependencygraph.NewGraphFromNodes(convertedNodes)

	assert.Nil(t, err)
	assert.NotNil(t, graph)
	executionPlan := graph.TopoSortedLayers()
	assert.NotNil(t, executionPlan)

	commChannel := make(chan *models.GraphCommChannel)
	go func() {
		cm.RunGraph(nodes, executionPlan, *socket, map[string]string{}, map[string]string{}, false, commChannel)
	}()

	for {
		c := <-commChannel
		if c == nil {
			break
		} else {
			assert.Nil(t, (*c).Err)
			fmt.Printf("Running step %d scenario: %s\n", *c.Layer, *c.ScenarioId)
		}

	}

}

func TestScenarioOrchestrator_ListRunningScenarios(t *testing.T) {
	conf := GetOriginalConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
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
	containerMap, err := cm.ListRunningContainers(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, containerMap)
}

func TestScenarioOrchestrator_GetScenarioDetail(t *testing.T) {
	conf := GetOriginalConfig()
	cm := ScenarioOrchestrator{
		Config: conf,
	}
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
	scenarios, err := cm.ListRunningContainers(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	for _, v := range *scenarios {
		containerMap, err := cm.InspectRunningScenario(v, *socket)
		assert.Nil(t, err)
		assert.NotNil(t, containerMap)
	}

}
