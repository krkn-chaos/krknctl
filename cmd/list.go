package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/spf13/cobra"
	"log"
)

func NewListCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: "lists available scenarios and running scenarios",
		Long:  `lists available scenarios and running scenarios`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return command
}

func NewListScenariosCommand(factory *providerfactory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "available",
		Short: "lists available scenarios",
		Long:  `list available krkn-hub scenarios`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registrySettings, err := models.NewRegistryV2FromEnv(config)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return err
				}
			}

			if err != nil {
				return err
			}
			privateRegistry := false
			if registrySettings != nil {
				privateRegistry = true
			}
			provider := GetProvider(privateRegistry, factory)
			s := NewSpinnerWithSuffix("fetching scenarios...", registrySettings)
			s.Start()
			scenarios, err := provider.GetRegistryImages(registrySettings)
			if err != nil {
				s.Stop()
				log.Fatalf("failed to fetch scenarios: %v", err)
			}
			s.Stop()
			scenarioTable := NewScenarioTable(scenarios, privateRegistry)
			scenarioTable.Print()
			fmt.Print("\n")
			return nil
		},
	}
	return command
}

func NewListRunningScenario(scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator) *cobra.Command {
	var command = &cobra.Command{
		Use:   "running",
		Short: "lists running scenarios",
		Long:  `list running krkn-hub scenarios`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			(*scenarioOrchestrator).PrintContainerRuntime()
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}
			runningScenarios, err := (*scenarioOrchestrator).ListRunningScenarios(ctx)

			if err != nil {
				return err
			}
			if len(*runningScenarios) == 0 {
				_, err = color.New(color.FgYellow).Println("No scenarios are currently running.")
				if err != nil {
					return err
				}
				return nil
			}
			// TODO: Consider no result message (everywhere ideally)
			table := NewRunningScenariosTable(*runningScenarios)
			table.Print()
			return nil
		},
	}
	return command
}
