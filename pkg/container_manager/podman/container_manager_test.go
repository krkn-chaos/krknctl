package podman

import (
	"encoding/json"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
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

func getGraphConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:                    "0.0.1",
		QuayProtocol:               "https",
		QuayHost:                   "quay.io",
		QuayOrg:                    "krkn-chaos",
		QuayRegistry:               "krknctl-test",
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
	containerId, err := cm.RunAttached(conf.GetQuayImageUri()+":"+scenario.Name, scenario.Name, *socket, env, false, map[string]string{}, os.Stdout, os.Stderr)
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
			"END":"7"
		},
		"volumes":{}
	},
	"first-row-1":{
		"depends_on":"root",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"7"
		},
		"volumes":{}
	},
	"first-row-2":{
		"depends_on":"root",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"7"
		},
		"volumes":{}
	},
	"second-row":{
		"depends_on":"first-row-1",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"7"
		},
		"volumes":{}
	},
	"third-row-1":{
		"depends_on":"second-row",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"7"
		},
		"volumes":{}
	},
	"third-row-2":{
		"depends_on":"second-row",
		"image":"quay.io/krkn-chaos/krknctl-test:dummy-scenario",
		"name":"dummy-scenario",
		"env":{
			"END":"7"
		},
		"volumes":{}
	}
}
`
	conf := getGraphConfig()
	cm := ContainerManager{
		Config: conf,
	}
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{}
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", conf.GetQuayRepositoryApiUri())
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

	nodes := make(map[string]container_manager.ScenarioNode)
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

	commChannel := make(chan *container_manager.CommChannel)
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
