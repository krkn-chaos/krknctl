package docker

import (
	"context"
	"fmt"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/scenarioorchestratortest"
	"github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestScenarioOrchestrator_Docker_Connect(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorConnect(t, &sopodman)
}

func TestScenarioOrchestrator_Docker_RunAttached(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorRunAttached(t, &sopodman, config, 3)
}

func TestScenarioOrchestrator_Docker_Run(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorRun(t, &sodocker, config, 5)
}

func TestScenarioOrchestrator_Docker_RunGraph(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorRunGraph(t, &sodocker, config)
}

func findContainers(t *testing.T, config config.Config, ctx context.Context) []string {
	scenarioNameRegex, err := regexp.Compile(fmt.Sprintf("%s-.*-([0-9]+)", config.ContainerPrefix))
	assert.Nil(t, err)

	cli, err := dockerClientFromContext(ctx)
	assert.Nil(t, err)

	// Recuperare i container attualmente in esecuzione
	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	assert.Nil(t, err)
	var foundContainers []string
	for _, container := range containers {
		if scenarioNameRegex.MatchString(container.Names[0]) {
			foundContainers = append(foundContainers, container.Names[0])
		}
	}
	return foundContainers
}

func TestScenarioOrchestrator_Docker_CleanContainers(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	envuid := os.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}
	socket, err := sodocker.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	ctx, err := sodocker.Connect(*socket)
	assert.Nil(t, err)
	scenarioorchestratortest.CommonTestScenarioOrchestratorRunAttached(t, &sodocker, config, 5)
	foundContainers := findContainers(t, config, ctx)
	assert.Greater(t, len(foundContainers), 0)
	numcontainers, err := sodocker.CleanContainers(ctx)
	assert.Nil(t, err)
	assert.Equal(t, len(foundContainers), *numcontainers)
	foundContainers = findContainers(t, config, ctx)
	assert.Equal(t, len(foundContainers), 0)

}

func TestScenarioOrchestrator_Docker_AttachWait(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	fileContent := scenarioorchestratortest.CommonAttachWait(t, &sopodman, config)
	fmt.Println("FILE CONTENT -> ", fileContent)
	assert.True(t, strings.Contains(fileContent, "Release the krkn 4"))

}

func TestScenarioOrchestrator_Docker_Kill(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorKillContainers(t, &sodocker, config)
}

func TestScenarioOrchestrator_Docker_ListRunningContainers(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorListRunningContainers(t, &sodocker, config)
}

func TestScenarioOrchestrator_Docker_ListRunningScenarios(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorListRunningScenarios(t, &sodocker, config)
}
func TestScenarioOrchestrator_Docker_InspectRunningScenario(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorInspectRunningScenario(t, &sodocker, config)

}

func TestScenarioOrchestrator_Docker_GetContainerRuntimeSocket(t *testing.T) {
	config := scenarioorchestratortest.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	var uid *int = nil
	envuid := os.Getenv("USERID")
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	} else {
		_uid := 1337
		uid = &_uid
	}
	assert.NotNil(t, uid)

	socket, err := sodocker.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	assert.Equal(t, config.DockerSocketRoot, *socket)

}

func TestScenarioOrchestrator_Docker_GetContainerRuntime(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	assert.Equal(t, sopodman.GetContainerRuntime(), models.Docker)
}

func TestScenarioOrchestrator_Docker_ResolveContainerId(t *testing.T) {
	config := scenarioorchestratortest.CommonGetTestConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	scenarioorchestratortest.CommonTestScenarioOrchestratorResolveContainerName(t, &sodocker, config, 3)
}
