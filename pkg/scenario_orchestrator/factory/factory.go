package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/docker"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/podman"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
)

type ScenarioOrchestratorFactory struct {
	Config config.Config
}

func NewScenarioOrchestratorFactory(config config.Config) *ScenarioOrchestratorFactory {
	return &ScenarioOrchestratorFactory{
		Config: config,
	}
}

func (f *ScenarioOrchestratorFactory) NewInstance(containerEnvironment models.ContainerRuntime) scenario_orchestrator.ScenarioOrchestrator {
	switch containerEnvironment {
	case models.Podman:
	case models.Docker:
		return f.getOrchestratorInstance(containerEnvironment)
	case models.Both:
		defaultContainerEnvironment := utils.EnvironmentFromString(f.Config.DefaultContainerPlatform)
		return f.getOrchestratorInstance(defaultContainerEnvironment)
	}
	return nil
}

func (f *ScenarioOrchestratorFactory) getOrchestratorInstance(containerEnvironment models.ContainerRuntime) scenario_orchestrator.ScenarioOrchestrator {
	if containerEnvironment == models.Podman {
		return &podman.ScenarioOrchestrator{
			Config:           f.Config,
			ContainerRuntime: containerEnvironment,
		}
	} else {
		return &docker.ScenarioOrchestrator{
			Config:           f.Config,
			ContainerRuntime: containerEnvironment,
		}
	}
}
