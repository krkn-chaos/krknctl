// Package factory provides the factory for the various container runtime implementations
package factory

import (
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/docker"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/podman"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
)

type ScenarioOrchestratorFactory struct {
	Config config.Config
}

func NewScenarioOrchestratorFactory(config config.Config) *ScenarioOrchestratorFactory {
	return &ScenarioOrchestratorFactory{
		Config: config,
	}
}

func (f *ScenarioOrchestratorFactory) NewInstance(containerEnvironment models.ContainerRuntime) scenarioorchestrator.ScenarioOrchestrator {
	switch containerEnvironment {
	case models.Podman:
		return f.getOrchestratorInstance(models.Podman)
	case models.Docker:
		return f.getOrchestratorInstance(models.Docker)
	case models.Both:
		defaultContainerEnvironment := utils.EnvironmentFromString(f.Config.DefaultContainerPlatform)
		return f.getOrchestratorInstance(defaultContainerEnvironment)
	}
	return nil
}

func (f *ScenarioOrchestratorFactory) getOrchestratorInstance(containerEnvironment models.ContainerRuntime) scenarioorchestrator.ScenarioOrchestrator {
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
