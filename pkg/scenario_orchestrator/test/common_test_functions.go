package test

import (
	"encoding/json"
	"errors"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	provider_models "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"os/user"
	"strconv"
	"strings"
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
	conf.QuayScenarioRegistry = "krknctl-test"
	return conf
}

func CommonTestScenarioOrchestratorRun(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config, duration int) string {
	env := map[string]string{
		"END": fmt.Sprintf("%d", duration),
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)

	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
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
	containerId, err := so.Run(registryUri+":"+scenario.Name, containerName, env, false, map[string]string{}, nil, ctx, nil)
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

	//pulling image from private registry with token
	quayToken := os.Getenv("QUAY_TOKEN")
	pr := provider_models.RegistryV2{
		RegistryUrl:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTls:            true,
	}

	timestamp = time.Now().Unix()
	containerName = fmt.Sprintf("%s-%s-%d%d", conf.ContainerPrefix, scenario.Name, timestamp, rand.Int())
	containerId, err = so.Run(pr.GetPrivateRegistryUri()+":"+scenario.Name, containerName, env, false, map[string]string{}, nil, ctx, &pr)
	if so.GetContainerRuntime() == models.Docker {
		if err != nil {
			fmt.Println(err.Error())
		}
		assert.Nil(t, err)
		assert.NotNil(t, containerId)
	} else {
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "token authentication not yet supported in podman")
	}

	//pulling image from private registry with username and password
	basicAuthUsername := "testuser"
	basicAuthPassword := "testpassword"

	pr = provider_models.RegistryV2{
		RegistryUrl:        "localhost:5001",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		Username:           &basicAuthUsername,
		Password:           &basicAuthPassword,
		SkipTls:            true,
	}

	timestamp = time.Now().Unix()
	containerName = fmt.Sprintf("%s-%s-%d%d", conf.ContainerPrefix, scenario.Name, timestamp, rand.Int())
	containerId, err = so.Run(pr.GetPrivateRegistryUri()+":"+scenario.Name, containerName, env, false, map[string]string{}, nil, ctx, &pr)
	if err != nil {
		fmt.Println(err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

	return *containerId
}

func CommonTestScenarioOrchestratorRunAttached(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config, duration int) string {
	env := map[string]string{
		"END":         fmt.Sprintf("%d", duration),
		"EXIT_STATUS": "0",
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail("failing-scenario", nil)
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
	containerName1 := utils.GenerateContainerName(conf, scenario.Name, nil)
	containerId, err := so.RunAttached(registryUri+":"+scenario.Name, containerName1, env, false, map[string]string{}, os.Stdout, os.Stderr, nil, ctx, nil)
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

	// Testing exit status > 0
	exitStatus := 3
	env["END"] = fmt.Sprintf("%d", duration)
	env["EXIT_STATUS"] = fmt.Sprintf("%d", exitStatus)
	containerName2 := utils.GenerateContainerName(conf, scenario.Name, nil)
	containerId, err = so.RunAttached(registryUri+":"+scenario.Name, containerName2, env, false, map[string]string{}, os.Stdout, os.Stderr, nil, ctx, nil)
	if err != nil {
		fmt.Println("ERROR -> " + err.Error())
	}
	assert.NotNil(t, err)
	assert.NotNil(t, containerId)
	var staterr *utils.ExitError
	assert.True(t, errors.As(err, &staterr))
	assert.NotNil(t, staterr)
	assert.Equal(t, staterr.ExitStatus, exitStatus)

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
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: config,
		}}
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
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
		so.RunGraph(nodes, executionPlan, map[string]string{}, map[string]string{}, false, commChannel, nil, uid)
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

	data = `
{
  "dummy-scenario-eoghanacht": {
    "_comment": "I'm the root Node!",
    "image": "quay.io/krkn-chaos/krkn-hub:dummy-scenario",
    "name": "dummy-scenario",
    "env": {
      "END": "1",
      "EXIT_STATUS": "0"
    }
  },
  "dummy-scenario-ganglionic": {
    "image": "quay.io/krkn-chaos/krkn-hub:dummy-scenario",
    "name": "dummy-scenario",
    "env": {
      "END": "3",
      "EXIT_STATUS": "0"
    },
    "depends_on": "dummy-scenario-eoghanacht"
  },
  "dummy-scenario-mirthsome": {
    "image": "quay.io/krkn-chaos/krkn-hub:dummy-scenario",
    "name": "dummy-scenario",
    "env": {
      "END": "1",
      "EXIT_STATUS": "%d"
    },
    "depends_on": "dummy-scenario-eoghanacht"
  },
  "dummy-scenario-stanly": {
    "image": "quay.io/krkn-chaos/krkn-hub:dummy-scenario",
    "name": "dummy-scenario",
    "env": {
      "END": "1",
      "EXIT_STATUS": "0"
    },
    "depends_on": "dummy-scenario-mirthsome"
  }
}`
	exitStatus := 3
	data = fmt.Sprintf(data, exitStatus)
	nodes = make(map[string]models.ScenarioNode)
	err = json.Unmarshal([]byte(data), &nodes)
	assert.Nil(t, err)

	convertedNodes = make(map[string]dependencygraph.ParentProvider, len(nodes))

	// Populate the new map
	for key, node := range nodes {
		// Since ScenarioNode implements ParentProvider, this is valid
		convertedNodes[key] = node
	}

	graph, err = dependencygraph.NewGraphFromNodes(convertedNodes)

	assert.Nil(t, err)
	assert.NotNil(t, graph)
	executionPlan = graph.TopoSortedLayers()
	assert.NotNil(t, executionPlan)

	commChannel = make(chan *models.GraphCommChannel)
	go func() {
		so.RunGraph(nodes, executionPlan, map[string]string{}, map[string]string{}, false, commChannel, nil, uid)
	}()

	for {
		c := <-commChannel
		if c == nil {
			break
		} else {
			if (*c).Err != nil {
				assert.NotNil(t, (*c).ScenarioId)
				assert.NotNil(t, (*c).ScenarioLogFile)
				assert.NotNil(t, (*c).Layer)
				var staterr *utils.ExitError
				assert.True(t, errors.As(c.Err, &staterr))
				assert.NotNil(t, staterr)
				assert.Equal(t, staterr.ExitStatus, exitStatus)
			}

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
		containerMap, err := so.InspectScenario(v, ctx)
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
	testFilename := fmt.Sprintf("krknctl-attachwait-%d", time.Now().Unix())
	fmt.Println("FILE_NAME -> ", testFilename)
	file, err := os.OpenFile(testFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	assert.Nil(t, err)
	containerId := CommonTestScenarioOrchestratorRunAttached(t, so, conf, 5)
	so.AttachWait(&containerId, file, file, ctx)
	err = file.Close()
	assert.Nil(t, err)
	filecontent, err := os.ReadFile(testFilename)
	assert.Nil(t, err)
	return string(filecontent)
}

func CommonTestScenarioOrchestratorResolveContainerName(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config, duration int) {
	env := map[string]string{
		"END":         fmt.Sprintf("%d", duration),
		"EXIT_STATUS": "0",
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)
	scenario, err := quayProvider.GetScenarioDetail("failing-scenario", nil)
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
	containerId, err := so.RunAttached(registryUri+":"+scenario.Name, containerName, env, false, map[string]string{}, os.Stdout, os.Stderr, nil, ctx, nil)
	assert.Nil(t, err)
	assert.NotNil(t, containerId)

	resolvedContainerId, err := so.ResolveContainerName(containerName, ctx)
	assert.Nil(t, err)
	assert.Equal(t, *containerId, *resolvedContainerId)

	resolvedContainerId, err = so.ResolveContainerName("not_found", ctx)
	assert.Nil(t, resolvedContainerId)
	assert.Nil(t, err)

}

func CommonTestScenarioOrchestratorKillContainers(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config) {
	env := map[string]string{
		"END": "20",
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)

	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
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
	containerName := fmt.Sprintf("%s-%s-kill-%d", conf.ContainerPrefix, scenario.Name, timestamp)
	containerId, err := so.Run(registryUri+":"+scenario.Name, containerName, env, false, map[string]string{}, nil, ctx, nil)
	time.Sleep(2 * time.Second)
	containers, err := so.ListRunningContainers(ctx)
	assert.Nil(t, err)
	found := false
	for _, v := range *containers {
		if strings.Contains(v.Name, "kill") {
			found = true
		}
	}
	assert.True(t, found)

	err = so.Kill(containerId, ctx)
	assert.Nil(t, err)
	time.Sleep(2 * time.Second)
	containers, err = so.ListRunningContainers(ctx)
	assert.Nil(t, err)
	found = false
	for _, v := range *containers {
		if strings.Contains(v.Name, "kill") {
			found = true
		}
	}
	assert.False(t, found)
	end := time.Now().Unix()
	assert.True(t, end-timestamp < 20)
}

func CommonTestScenarioOrchestratorListRunningScenarios(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config) {
	env := map[string]string{
		"END": "20",
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)

	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
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

	containerName1 := utils.GenerateContainerName(conf, scenario.Name, nil)
	containerName2 := utils.GenerateContainerName(conf, scenario.Name, nil)

	//starting containers in inverted order to check if lisRunningScenarios returns them sorted
	sortedContainers := make(map[int]string)
	_, err = so.Run(registryUri+":"+scenario.Name, containerName2, env, false, map[string]string{}, nil, ctx, nil)
	_, err = so.Run(registryUri+":"+scenario.Name, containerName1, env, false, map[string]string{}, nil, ctx, nil)
	time.Sleep(1 * time.Second)

	assert.Nil(t, err)
	runningContainers, err := so.ListRunningScenarios(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, runningContainers)
	i := 0
	for _, r := range *runningContainers {
		if r.Container.Name == containerName1 || r.Container.Name == containerName2 {
			sortedContainers[i] = r.Container.Name
			i++
		}
	}
	assert.Nil(t, err)
	fmt.Println(sortedContainers)
	assert.True(t, len(*runningContainers) >= 2)
	assert.Equal(t, sortedContainers[0], containerName1)
	assert.Equal(t, sortedContainers[1], containerName2)
}

func CommonTestScenarioOrchestratorInspectRunningScenario(t *testing.T, so scenario_orchestrator.ScenarioOrchestrator, conf krknctlconfig.Config) {
	env := map[string]string{
		"END": "20",
	}

	currentUser, err := user.Current()
	fmt.Println("Current user: " + (*currentUser).Name)
	fmt.Println("current user id" + (*currentUser).Uid)
	quayProvider := quay.ScenarioProvider{
		BaseScenarioProvider: provider.BaseScenarioProvider{
			Config: conf,
		}}
	registryUri, err := conf.GetQuayImageUri()
	assert.Nil(t, err)

	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
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

	containerName1 := utils.GenerateContainerName(conf, scenario.Name, nil)
	containerId1, err := so.Run(registryUri+":"+scenario.Name, containerName1, env, false, map[string]string{}, nil, ctx, nil)
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)

	inspectData, err := so.InspectScenario(models.Container{Id: *containerId1}, ctx)
	assert.Nil(t, err)
	assert.NotNil(t, inspectData)

	assert.Equal(t, inspectData.Container.Name, containerName1)
	assert.Equal(t, inspectData.Container.Id, *containerId1)
	assert.Equal(t, inspectData.Scenario.Name, scenario.Name)

	inspectData, err = so.InspectScenario(models.Container{Id: "mimmo"}, ctx)
	assert.Nil(t, err)
	assert.Nil(t, inspectData)

}
