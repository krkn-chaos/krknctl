package cmd

import (
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	scenarioorchestratorfactory "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func getOrchestrator(t *testing.T) scenarioorchestrator.ScenarioOrchestrator {
	config := getConfig(t)
	scenarioOrchestratorFactory := scenarioorchestratorfactory.NewScenarioOrchestratorFactory(config)
	detectedRuntime, err := utils.DetectContainerRuntime(config)
	assert.Nil(t, err)
	scenarioOrchestrator := scenarioOrchestratorFactory.NewInstance(*detectedRuntime)
	assert.NotNil(t, scenarioOrchestrator)
	return scenarioOrchestrator

}

func getProviderFactory(t *testing.T) *providerfactory.ProviderFactory {
	config := getConfig(t)
	providerFactory := providerfactory.NewProviderFactory(&config)
	return providerFactory
}

func TestNewAttachCmd(t *testing.T) {
	orchestrator := getOrchestrator(t)
	cmd := NewAttachCmd(&orchestrator)
	assert.NotNil(t, cmd)
}

func TestNewCleanCommand(t *testing.T) {
	orchestrator := getOrchestrator(t)
	cmd := NewCleanCommand(&orchestrator, getConfig(t))
	assert.NotNil(t, cmd)
}

func TestNewDescribeCommand(t *testing.T) {
	providerFactory := getProviderFactory(t)
	cmd := NewDescribeCommand(providerFactory, getConfig(t))
	assert.NotNil(t, cmd)
}

func TestNewGraphCommand(t *testing.T) {
	cmd := NewGraphCommand()
	assert.NotNil(t, cmd)
}

func TestNewListCommand(t *testing.T) {

	cmd := NewListCommand()
	assert.NotNil(t, cmd)
}

func TestNewQueryStatusCommand(t *testing.T) {
	orchestrator := getOrchestrator(t)
	cmd := NewQueryStatusCommand(&orchestrator)
	assert.NotNil(t, cmd)
}

func TestNewRandomCommand(t *testing.T) {
	cmd := NewRandomCommand()
	assert.NotNil(t, cmd)
}

func TestNewRootCommand(t *testing.T) {
	config := getConfig(t)
	rootCmd := NewRootCommand(config)
	assert.NotNil(t, rootCmd)
}

func TestNewRunCommand(t *testing.T) {
	config := getConfig(t)
	providerFactory := getProviderFactory(t)
	scenarioOrchestrator := getOrchestrator(t)
	cmd := NewRunCommand(providerFactory, &scenarioOrchestrator, config)
	assert.NotNil(t, cmd)
}
