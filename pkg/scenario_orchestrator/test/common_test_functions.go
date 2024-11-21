package test

import (
	"encoding/json"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/letsencrypt/boulder/core"
	"github.com/stretchr/testify/assert"
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"
)

func CommonGetConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func CommonGetTestConfig(t *testing.T) krknctlconfig.Config {
	conf := CommonGetConfig(t)
	conf.QuayRegistry = "krknctl-test"
	return conf
}

func CommonTestScenarioOrchestratorRun(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config, duration int) string {
	env := map[string]string{
		"END": fmt.Sprintf("%d", duration),
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &conf}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)

	apiUri, err := conf.GetQuayRepositoryApiUri()
	assert.Nil(t, err)

	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", apiUri)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
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
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

	fmt.Println("CONTAINER SOCKET -> " + *socket)
	timestamp := time.Now().Unix()
	containerName := fmt.Sprintf("%s-%s-%d", conf.ContainerPrefix, scenario.Name, timestamp)
	containerId, err := so.Run(registryUri+":"+scenario.Name, containerName, env, false, map[string]string{}, nil, ctx)
	assert.Nil(t, err)
	assert.NotNil(t, containerId)
	return *containerId
}

func CommonTestScenarioOrchestratorRunAttached(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config, duration int) string {
	env := map[string]string{
		"END": fmt.Sprintf("%d", duration),
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &conf}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)
	apiUri, err := conf.GetQuayRepositoryApiUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", apiUri)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
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
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

	fmt.Println("CONTAINER SOCKET -> " + *socket)
	containerName := utils.GenerateContainerName(conf, scenario.Name, nil)
	containerId, err := so.RunAttached(registryUri+":"+scenario.Name, containerName, env, false, map[string]string{}, os.Stdout, os.Stderr, nil, ctx)
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)
	return *containerId
}

func CommonTestScenarioOrchestratorConnect(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, config krknctlconfig.Config) {
	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)

	envuid := os.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	assert.Nil(t, err)
	fmt.Println("CONTAINER SOCKET -> " + *socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)
}

func CommonTestScenarioOrchestratorRunGraph(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, config krknctlconfig.Config) {
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

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{Config: &config}
	repositoryApi, err := config.GetQuayRepositoryApiUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", repositoryApi)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	kubeconfig, err := utils.PrepareKubeconfig(nil, config)
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
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

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
		so.RunGraph(nodes, executionPlan, map[string]string{}, map[string]string{}, false, commChannel, ctx)
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

func CommonTestScenarioOrchestratorListRunningContainers(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, config krknctlconfig.Config) {
	kubeconfig, err := utils.PrepareKubeconfig(nil, config)
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
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

	containers, err := so.ListRunningContainers(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, containers)
}

func CommonScenarioDetail(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator) {
	envuid := os.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}

	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)
	scenarios, err := so.ListRunningContainers(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	for _, v := range *scenarios {
		containerMap, err := so.InspectRunningScenario(v, ctx)
		assert.Nil(t, err)
		assert.NotNil(t, containerMap)
	}
}

func CommonAttachWait(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config) string {
	envuid := os.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}
	socket, err := so.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.NotNil(t, socket)
	ctx, err := so.Connect(*socket)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)
	testFilename := fmt.Sprintf("krknctl-attachwait-%s-%d", core.RandomString(5), time.Now().Unix())
	fmt.Println("FILE_NAME -> ", testFilename)
	file, err := os.OpenFile(testFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	assert.Nil(t, err)
	containerId := CommonTestScenarioOrchestratorRunAttached(t, so, conf, 10)
	so.AttachWait(&containerId, file, file, ctx)
	err = file.Close()
	assert.Nil(t, err)
	filecontent, err := os.ReadFile(testFilename)
	assert.Nil(t, err)
	return string(filecontent)
}
