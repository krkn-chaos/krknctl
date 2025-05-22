package factory

import (
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/docker"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/podman"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestScenarioOrchestratorFactory_NewInstance(t *testing.T) {
	typeProviderPodman := &podman.ScenarioOrchestrator{}
	typeProviderDocker := &docker.ScenarioOrchestrator{}
	conf, err := config.LoadConfig()
	assert.Nil(t, err)
	assert.NotNil(t, conf)
	factory := NewScenarioOrchestratorFactory(conf)
	assert.NotNil(t, factory)
	scenarioDocker := factory.NewInstance(models.Docker)
	assert.NotNil(t, scenarioDocker)
	assert.IsType(t, scenarioDocker, typeProviderDocker)

	scenarioPodman := factory.NewInstance(models.Podman)
	assert.NotNil(t, scenarioPodman)
	assert.IsType(t, scenarioPodman, typeProviderPodman)

}
