package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/spf13/cobra"
)

func NewCleanCommand(scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "cleans already run scenario files and containers",
		Long:  `cleans already run scenario files and containers`,
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
			deletedContainers, err := (*scenarioOrchestrator).CleanContainers(ctx)
			if err != nil {
				return err
			}
			deletedKubeconfigFiles, err := utils.CleanKubeconfigFiles(config)
			if err != nil {
				return err
			}
			deletedLogFiles, err := utils.CleanLogFiles(config)
			if err != nil {
				return err
			}
			fmt.Printf("%d containers, %d kubeconfig files, %d log files deleted\n", *deletedContainers, *deletedKubeconfigFiles, *deletedLogFiles)
			return nil
		},
	}

	return command
}
