package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/spf13/cobra"
	"os"
	"path"
)

func resolveContainerIDOrName(orchestrator scenario_orchestrator.ScenarioOrchestrator, arg string, conn context.Context) error {
	var scenarioContainer *models.ScenarioContainer
	var containerID *string

	containerID, err := orchestrator.ResolveContainerName(arg, conn)
	if err != nil {
		return err
	}
	if containerID == nil {
		containerID = &arg
	}

	scenarioContainer, err = orchestrator.InspectScenario(models.Container{Id: *containerID}, conn)

	if err != nil {
		return err
	}

	if scenarioContainer == nil {
		return fmt.Errorf("scenarioContainer with id or name %s not found", arg)
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(scenarioContainer.Container)
	if err != nil {
		return err
	}

	fmt.Println(buf.String())
	if scenarioContainer.Container.ExitStatus != 0 {
		return &utils.ExitError{ExitStatus: scenarioContainer.Container.ExitStatus}
	}
	return nil
}

func resolveGraphFile(orchestrator scenario_orchestrator.ScenarioOrchestrator, filename string, conn context.Context) error {
	var scenarioFile = make(map[string]providermodels.ScenarioDetail)
	var containers = make([]models.Container, 0)
	var statusCodeError error
	fileData, err := os.ReadFile(path.Clean(filename))
	if err != nil {
		return err
	}
	err = json.Unmarshal(fileData, &scenarioFile)
	if err != nil {
		return err
	}
	for key := range scenarioFile {
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
					if (*containerScenario).Container.ExitStatus != 0 {
						return &utils.ExitError{ExitStatus: (*containerScenario).Container.ExitStatus}
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(containers)
	if err != nil {
		return err
	}
	fmt.Println(buf.String())
	return statusCodeError
}

func NewQueryStatusCommand(scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator) *cobra.Command {
	var command = &cobra.Command{
		Use:          "query-status",
		Short:        "checks the status of a container or a list of containers",
		Long:         `checks the status of a container or a list of containers by container name or container ID`,
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
				err = resolveContainerIDOrName(*scenarioOrchestrator, args[0], conn)
				var staterr *utils.ExitError
				if errors.As(err, &staterr) {
					if staterr.ExitStatus != 0 {
						os.Exit(staterr.ExitStatus)
					}
				}
				return err
			}

			graphPath, err := cmd.Flags().GetString("graph")
			if err != nil {
				return err
			}

			if graphPath == "" {
				return fmt.Errorf("neither container ID or name nor graph plan file specified")
			}

			if !CheckFileExists(graphPath) {
				return fmt.Errorf("graph file %s not found", graphPath)
			}

			err = resolveGraphFile(*scenarioOrchestrator, graphPath, conn)
			// since multiple errors of different values might be set
			// on graph it exits with a generic 1
			var staterr *utils.ExitError
			if errors.As(err, &staterr) {
				if staterr.ExitStatus != 0 {
					os.Exit(1)
				}
			}

			if err != nil {
				return err
			}
			return nil
		},
	}
	return command
}
