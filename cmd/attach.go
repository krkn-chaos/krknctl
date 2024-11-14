package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
	"time"
)

func customCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Esempio di mappa di ID e descrizioni
	completions := map[string]string{
		"0": "questo rappresenta l'id del container 0",
		"1": "questo rappresenta l'id del container 1",
		"2": "questo rappresenta l'id del container 2",
		"3": "questo rappresenta l'id del container 3",
	}

	var results []string
	for id, description := range completions {
		if strings.HasPrefix(id, toComplete) {
			// Formatta l'output per mostrare l'ID seguito dalla descrizione
			results = append(results, fmt.Sprintf("%s\t%s", id, description))
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

func NewAttachCmd(scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
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
			scenarios, err := (*scenarioOrchestrator).ListRunningScenarios(*socket)
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

			runningScenarios, err := (*scenarioOrchestrator).ListRunningScenarios(*socket)
			if err != nil {
				return err
			}

			scenarioId, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid scenario id: %s, scenario id must be a number", args[0])
			}

			if scenarioId > int64(len(*runningScenarios)-1) {
				return fmt.Errorf("invalid scenario id: %d, scenario id out of range", scenarioId)
			}
			_, err = color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to stop streaming scenario output (scenario won't be interrupted)")
			if err != nil {
				return err
			}
			scenario := (*runningScenarios)[scenarioId]
			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}
			interrupted, err := (*scenarioOrchestrator).AttachWait(&scenario.Container.Id, &ctx, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}
			if *interrupted {
				_, err = color.New(color.FgRed, color.Underline).Println(fmt.Sprintf("scenario output terminated, container %s still running", scenario.Container.Id))
				if err != nil {
					return err
				}
			}
			return nil

		},
	}
	return command
}
