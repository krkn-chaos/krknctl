package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
	"time"
)

func NewAttachCmd(scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator) *cobra.Command {
	var command = &cobra.Command{
		Use:   "attach",
		Short: "connects krknctl to the running scenario logs",
		Long:  `connects krknctl to the running scenario logs`,
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			scenarios, err := (*scenarioOrchestrator).ListRunningScenarios(ctx)
			if err != nil || scenarios == nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			completions := make(map[string]string)

			for i, scenario := range *scenarios {
				start := time.Unix(scenario.Container.Started, 0)
				completions[fmt.Sprintf("%d", i)] = fmt.Sprintf("%s started %s ago", scenario.Container.Name, time.Since(start))
			}
			var results []string
			for id, description := range completions {
				if strings.HasPrefix(id, toComplete) {
					results = append(results, fmt.Sprintf("%s\t%s", id, description))
				}
			}

			return results, cobra.ShellCompDirectiveNoFileComp

		},
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

			scenarioID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid scenario id: %s, scenario id must be a number", args[0])
			}

			if scenarioID > int64(len(*runningScenarios)-1) {
				return fmt.Errorf("invalid scenario id: %d, scenario id out of range", scenarioID)
			}
			_, err = color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to stop streaming scenario output (scenario won't be interrupted)")
			if err != nil {
				return err
			}
			scenario := (*runningScenarios)[scenarioID]
			ctx, err = (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}
			interrupted, err := (*scenarioOrchestrator).AttachWait(&scenario.Container.ID, os.Stdout, os.Stderr, ctx)
			if err != nil {
				return err
			}
			if *interrupted {
				_, err = color.New(color.FgRed, color.Underline).Println(fmt.Sprintf("scenario output terminated, container %s still running", scenario.Container.ID))
				if err != nil {
					return err
				}
			}
			return nil

		},
	}
	return command
}
