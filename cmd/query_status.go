package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	provider_models "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/spf13/cobra"
	"os"
)

func resolveContainerIdOrName(orchestrator scenario_orchestrator.ScenarioOrchestrator, arg string, conn context.Context, conf config.Config) error {
	var scenarioContainer *models.ScenarioContainer
	var containerId *string

	containerId, err := orchestrator.ResolveContainerName(arg, conn)
	if err != nil {
		return err
	}
	if containerId == nil {
		containerId = &arg
	}

	scenarioContainer, err = orchestrator.InspectScenario(models.Container{Id: *containerId}, conn)

	if err != nil {
		return err
	}

	if scenarioContainer == nil {
		return fmt.Errorf("scenarioContainer with id or name %s not found", arg)
	}

	containerJson, err := json.Marshal(scenarioContainer.Container)
	if err != nil {
		return err
	}
	fmt.Println(string(containerJson))
	if scenarioContainer.Container.ExitStatus != 0 {
		return fmt.Errorf("%s %d", conf.ContainerExitStatusPrefix, scenarioContainer.Container.ExitStatus)
	}
	return nil
}

func resolveGraphFile(orchestrator scenario_orchestrator.ScenarioOrchestrator, filename string, conn context.Context, conf config.Config) error {
	var scenarioFile = make(map[string]provider_models.ScenarioDetail)
	var containers = make([]models.Container, 0)

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(fileData, &scenarioFile)
	if err != nil {
		return err
	}
	for key, _ := range scenarioFile {
		scenario, err := orchestrator.ResolveContainerName(key, conn)
		if err != nil {
			return err
		}
		if scenario != nil {
			containerScenario, err := orchestrator.InspectScenario(models.Container{Id: *scenario}, conn)
			if err != nil {
				return err
			}
			if containerScenario != nil {
				if (*containerScenario).Container != nil {
					containers = append(containers, *(*containerScenario).Container)
				}
			}
		}
	}
	containersJson, err := json.Marshal(containers)
	if err != nil {
		return err
	}
	fmt.Println(string(containersJson))
	return nil
}

func NewQueryStatusCommand(scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:          "query-status",
		Short:        "checks the status of a container or a list of containers",
		Long:         `checks the status of a container or a list of containers by container name or container Id`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			conn, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				err = resolveContainerIdOrName(*scenarioOrchestrator, args[0], conn, config)
				return err
			}

			graphPath, err := cmd.Flags().GetString("graph")
			if err != nil {
				return err
			}

			if graphPath == "" {
				return fmt.Errorf("neither container Id or name nor graph plan file specified")
			}

			if CheckFileExists(graphPath) == false {
				return fmt.Errorf("graph file %s not found", graphPath)
			}

			err = resolveGraphFile(*scenarioOrchestrator, graphPath, conn, config)

			if err != nil {
				return err
			}
			return nil
		},
	}
	return command
}
