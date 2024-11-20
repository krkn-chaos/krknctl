package docker

import (
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/test"
	"testing"
)

func TestScenarioOrchestrator_Docker_Connect(t *testing.T) {
	config := test.CommonGetTestConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorConnect(t, &sopodman, config)
}

func TestScenarioOrchestrator_Docker_RunAttached(t *testing.T) {
	config := test.CommonGetConfig(t)
	sopodman := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorRunAttached(t, &sopodman, config)
}

func TestScenarioOrchestrator_Docker_Run(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorRun(t, &sodocker, config)
}

func TestScenarioOrchestrator_Docker_RunGraph(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorRunGraph(t, &sodocker, config)
}

func TestScenarioOrchestrator_Docker_CleanContainers(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_AttachWait(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_Attach(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_Kill(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_ListRunningContainers(t *testing.T) {
	config := test.CommonGetConfig(t)
	sodocker := ScenarioOrchestrator{Config: config, ContainerRuntime: models.Docker}
	test.CommonTestScenarioOrchestratorListRunningContainers(t, &sodocker, config)
}

func TestScenarioOrchestrator_Docker_ListRunningScenarios(t *testing.T) {

}
func TestScenarioOrchestrator_Docker_InspectRunningScenario(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_GetContainerRuntimeSocket(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_GetContainerRuntime(t *testing.T) {

}

func TestScenarioOrchestrator_Docker_PrintContainerRuntime(t *testing.T) {

}
