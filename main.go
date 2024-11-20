package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	scenarioorchestratorfactory "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"os"
)

func main() {
	config, err := krknctlconfig.LoadConfig()
	if err != nil {
		fmt.Printf("%s\n", color.New(color.FgHiRed).Sprint("failed to load configuration"))
		os.Exit(1)
	}
	scenarioOrchestratorFactory := scenarioorchestratorfactory.NewScenarioOrchestratorFactory(config)
	detectedRuntime, err := utils.DetectContainerRuntime(config)
	if err != nil {
		fmt.Printf("%s\n", color.New(color.FgHiRed).Sprint("failed to determine container runtime enviroment please install podman or docker and retry"))
		os.Exit(1)
	}

	scenarioOrchestrator := scenarioOrchestratorFactory.NewInstance(*detectedRuntime)
	providerFactory := providerfactory.NewProviderFactory(&config)

	cmd.Execute(providerFactory, &scenarioOrchestrator, config)
}
