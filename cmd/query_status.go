package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/config"
	provider_models "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/spf13/cobra"
	"os"
)

func resolveContainerIdOrName(orchestrator scenarioorchestrator.ScenarioOrchestrator, arg string, conn context.Context, conf config.Config) error {
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
		return &utils.ExitError{ExitStatus: int(scenarioContainer.Container.ExitStatus)}
	}
	return nil
}

func resolveGraphFile(orchestrator scenarioorchestrator.ScenarioOrchestrator, filename string, conn context.Context, conf config.Config) error {
	var scenarioFile = make(map[string]provider_models.ScenarioDetail)
	var containers = make([]models.Container, 0)
	var statusCodeError error
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
					if (*containerScenario).Container.ExitStatus != 0 {
						return &utils.ExitError{ExitStatus: int((*containerScenario).Container.ExitStatus)}
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

func NewQueryStatusCommand(scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
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
				return fmt.Errorf("neither container Id or name nor graph plan file specified")
			}

			if CheckFileExists(graphPath) == false {
				return fmt.Errorf("graph file %s not found", graphPath)
			}

			err = resolveGraphFile(*scenarioOrchestrator, graphPath, conn, config)
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
