package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	scenarioorchestratorfactory "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
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
	if scenarioOrchestrator == nil {
		fmt.Printf("%s\n", color.New(color.FgHiRed).Sprint("failed to build scenario orchestrator instance"))
		os.Exit(1)
	}
	providerFactory := providerfactory.NewProviderFactory(&config)

	cmd.Execute(providerFactory, &scenarioOrchestrator, config)
}
