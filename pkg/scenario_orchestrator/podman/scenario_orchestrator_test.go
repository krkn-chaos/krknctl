package podman

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/test"
	"github.com/stretchr/testify/assert"
	operating_system "os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestScenarioOrchestrator_Podman_Connect(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorConnect(t, &sopodman, config)
}

func TestScenarioOrchestrator_Podman_RunAttached(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRunAttached(t, &sopodman, config, 10)
}

func TestScenarioOrchestrator_Podman_Run(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRun(t, &sopodman, config, 5)
}

func TestScenarioOrchestrator_Podman_RunGraph(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRunGraph(t, &sopodman, config)
}

func findContainers(t *testing.T, config config.Config) (int, context.Context) {
	_true := true
	envuid := operating_system.Getenv("USERID")
	var uid *int = nil
	if envuid != "" {
		_uid, err := strconv.Atoi(envuid)
		assert.Nil(t, err)
		uid = &_uid
		fmt.Println("USERID -> ", *uid)
	}

	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	socket, err := sopodman.GetContainerRuntimeSocket(uid)
	assert.Nil(t, err)
	ctx, err := sopodman.Connect(*socket)
	assert.Nil(t, err)
	nameRegex, err := regexp.Compile(fmt.Sprintf("^%s.*-[0-9]+$", config.ContainerPrefix))
	assert.Nil(t, err)
	containers, err := containers.List(ctx, &containers.ListOptions{
		All: &_true,
	})
	assert.Nil(t, err)
	foundContainers := 0
	for _, container := range containers {
		if nameRegex.MatchString(container.Names[0]) {
			foundContainers++
		}
	}
	return foundContainers, ctx
}

func TestScenarioOrchestrator_Podman_CleanContainers(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRunAttached(t, &sopodman, config, 5)
	foundContainers, ctx := findContainers(t, config)
	assert.Greater(t, foundContainers, 0)
	numcontainers, err := sopodman.CleanContainers(ctx)
	assert.Nil(t, err)
	assert.Equal(t, foundContainers, *numcontainers)
	foundContainers, ctx = findContainers(t, config)
	assert.Equal(t, foundContainers, 0)
}

func TestScenarioOrchestrator_Podman_AttachWait(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	fileContent := test.CommonAttachWait(t, &sopodman, config)
	fmt.Println("FILE CONTENT -> ", fileContent)
	assert.True(t, strings.Contains(fileContent, "Release the krkn 4"))

}

func TestScenarioOrchestrator_Podman_Kill(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorKillContainers(t, &sopodman, config)

}

func TestScenarioOrchestrator_Podman_ListRunningContainers(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorListRunningContainers(t, &sopodman, config)
}

func TestScenarioOrchestrator_Podman_ListRunningScenarios(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorListRunningScenarios(t, &sopodman, config)
}
func TestScenarioOrchestrator_Podman_InspectRunningScenario(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorInspectRunningScenario(t, &sodocker, config)
}

func TestScenarioOrchestrator_Podman_GetContainerRuntimeSocket(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	var uid *int = nil
	envuid := operating_system.Getenv("USERID")
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
	switch os := runtime.GOOS; os {
	case "darwin":
		home, err := operating_system.UserHomeDir()
		assert.Nil(t, err)
		assert.NotNil(t, home)
		assert.Equal(t, fmt.Sprintf(config.PodmanDarwinSocketTemplate, home), *socket)
	case "linux":
		assert.Equal(t, fmt.Sprintf(config.PodmanLinuxSocketTemplate, *uid), *socket)
	default:
		panic("😱")
	}

}

func TestScenarioOrchestrator_Podman_GetContainerRuntime(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	assert.Equal(t, sopodman.GetContainerRuntime(), models.Podman)
}

func TestScenarioOrchestrator_Podman_ResolveContainerId(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorResolveContainerName(t, &sopodman, config, 3)
}
