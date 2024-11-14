package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
	"time"
)

func NewRunCommand(factory *factory.ProviderFactory, scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	collectedFlags := make(map[string]*string)
	var command = &cobra.Command{
		Use:                "run",
		Short:              "runs a scenario",
		Long:               `runs a scenario`,
		DisableFlagParsing: false,
		Args:               cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// TODO: datasource offline TBD
			/*
					offline, err := cmd.Flags().GetBool("offline")
					offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
					if err != nil {
								return []string{}, cobra.ShellCompDirectiveError
				    }
			*/

			dataSource := BuildDataSource(config, false, nil)
			provider := GetProvider(false, factory)

			scenarios, err := FetchScenarios(provider, dataSource)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			// TODO: datasource offline TBD
			/*
				offline, err := cmd.Flags().GetBool("offline")
				offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
				if err != nil {
					return err
				}
			*/
			dataSource := BuildDataSource(config, false, nil)
			provider := GetProvider(false, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0], dataSource)
			if err != nil {
				return err
			}
			if scenarioDetail == nil {
				return fmt.Errorf("%s scenario not found", args[0])
			}

			for _, field := range scenarioDetail.Fields {
				var defaultValue string = ""
				if field.Default != nil {
					defaultValue = *field.Default
				}
				collectedFlags[*field.Name] = cmd.LocalFlags().String(*field.Name, defaultValue, *field.Description)
				if err != nil {
					return err
				}

			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: datasource offline TBD
			/*
				offline, err := cmd.Flags().GetBool("offline")
				offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
				if err != nil {
					return err
				}
			*/

			(*scenarioOrchestrator).PrintContainerRuntime()
			spinner := NewSpinnerWithSuffix("validating input...")
			dataSource := BuildDataSource(config, false, nil)
			spinner.Start()
			runDetached := false

			provider := GetProvider(false, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0], dataSource)
			if err != nil {
				return err
			}
			spinner.Stop()

			if err != nil {
				return err
			}

			environment := make(map[string]string)
			volumes := make(map[string]string)
			var foundKubeconfig *string = nil
			var alertsProfile *string = nil
			var metricsProfile *string = nil
			for i, a := range args {
				if strings.HasPrefix(a, "--") {

					// since automatic flag parsing is disabled to allow dynamic flags
					// flags need to be parsed manually here eg. kubeconfig
					if a == "--kubeconfig" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						foundKubeconfig = &args[i+1]
					}
					if a == "--alerts-profile" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						alertsProfile = &args[i+1]
					}
					if a == "--metrics-profile" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						metricsProfile = &args[i+1]
					}

					if a == "--detached" {
						runDetached = true
					}
				}
			}

			kubeconfigPath, err := utils.PrepareKubeconfig(foundKubeconfig, config)
			if err != nil {
				return err
			}
			volumes[*kubeconfigPath] = config.KubeconfigPath
			if metricsProfile != nil {
				volumes[*metricsProfile] = config.MetricsProfilePath
			}

			if alertsProfile != nil {
				volumes[*alertsProfile] = config.AlertsProfilePath
			}
			//dynamic flags parsing
			for k, _ := range collectedFlags {
				field := scenarioDetail.GetFieldByName(k)
				if field == nil {
					return fmt.Errorf("field %s not found", k)
				}
				var foundArg *string = nil
				for i, a := range args {
					if a == fmt.Sprintf("--%s", k) {
						if len(args) < i+2 || strings.HasPrefix(args[i+1], "--") {
							return fmt.Errorf("%s has no value", args[i])
						}
						foundArg = &args[i+1]
					}
				}
				if field != nil {

					value, err := field.Validate(foundArg)
					if err != nil {
						return err
					}
					if value != nil && field.Type != typing.File {
						environment[*field.Variable] = *value
					} else if value != nil && field.Type == typing.File {
						fileSrcDst := strings.Split(*value, ":")
						volumes[fileSrcDst[0]] = fileSrcDst[1]
					}

					/*
						if value == nil && field.Required == false {
						fmt.Println(fmt.Sprintf("%s: nil default but not required", *field.Name))
					*/

				}

			}

			tbl := NewEnvironmentTable(environment)
			tbl.Print()
			fmt.Print("\n")

			//WIP
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			startTime := time.Now()
			containerName := utils.GenerateContainerName(config, scenarioDetail.Name, nil)
			if runDetached == false {
				_, err := color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to terminate the scenario")
				if err != nil {
					return err
				}
				_, err = (*scenarioOrchestrator).RunAttached(config.GetQuayImageUri()+":"+scenarioDetail.Name, containerName, *socket, environment, false, volumes, os.Stdout, os.Stderr)
				if err != nil {
					return err
				}
			} else {
				containerId, _, err := (*scenarioOrchestrator).Run(config.GetQuayImageUri()+":"+scenarioDetail.Name, containerName, *socket, environment, false, volumes)
				if err != nil {
					return err
				}
				_, err = color.New(color.FgGreen, color.Underline).Println(fmt.Sprintf("scenario %s started with containerId %s", scenarioDetail.Name, *containerId))
				if err != nil {
					return err
				}
			}
			scenarioDuration := time.Since(startTime)
			fmt.Println(fmt.Sprintf("%s ran for %s", scenarioDetail.Name, scenarioDuration.String()))
			return nil
		},
	}
	return command
}

func checkStringArgValue(args []string, index int) error {
	if len(args) < index+2 || strings.HasPrefix(args[index+1], "--") {
		return fmt.Errorf("%s has no value", args[index])
	}
	return nil
}
