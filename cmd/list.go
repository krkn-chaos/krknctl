package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	provider_factory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
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

func NewListScenariosCommand(factory *provider_factory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "available",
		Short: "lists available scenarios",
		Long:  `list available krkn-hub scenarios`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataSource := BuildDataSource(config, false, nil)
			provider := GetProvider(false, factory)
			s := NewSpinnerWithSuffix("fetching scenarios...")
			s.Start()
			scenarios, err := provider.GetScenarios(dataSource)
			if err != nil {
				s.Stop()
				log.Fatalf("failed to fetch scenarios: %v", err)
			}
			s.Stop()
			scenarioTable := NewScenarioTable(scenarios)
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
			runningScenarios, err := (*scenarioOrchestrator).ListRunningScenarios(*socket)
			if err != nil {
				return err
			}
			// TODO: Consider no result message (everywhere ideally)
			table := NewRunningScenariosTable(*runningScenarios)
			table.Print()
			return nil
		},
	}
	return command
}
