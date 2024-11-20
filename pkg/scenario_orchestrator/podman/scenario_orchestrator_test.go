package podman

import (
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/test"
	"testing"
)

func TestScenarioOrchestrator_Podman_Connect(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorConnect(t, &sopodman, config)
}

func TestScenarioOrchestrator_Podman_RunAttached(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRunAttached(t, &sopodman, config)
}

func TestScenarioOrchestrator_Podman_Run(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRun(t, &sopodman, config)
}

func TestScenarioOrchestrator_Podman_RunGraph(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorRunGraph(t, &sodocker, config)
}

func TestScenarioOrchestrator_Podman_CleanContainers(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_AttachWait(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_Attach(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_Kill(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_ListRunningContainers(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Podman}
	test.CommonTestScenarioOrchestratorListRunningContainers(t, &sodocker, config)
}

func TestScenarioOrchestrator_Podman_ListRunningScenarios(t *testing.T) {

}
func TestScenarioOrchestrator_Podman_InspectRunningScenario(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_GetContainerRuntimeSocket(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_GetContainerRuntime(t *testing.T) {

}

func TestScenarioOrchestrator_Podman_PrintContainerRuntime(t *testing.T) {

}
