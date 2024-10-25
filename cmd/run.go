package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

func NewRunCommand(factory *factory.ProviderFactory, containerManager *container_manager.ContainerManager, config config.Config) *cobra.Command {
	collectedFlags := make(map[string]*string)
	var runCmd = &cobra.Command{
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
			spinner := NewSpinnerWithSuffix("validating input...")
			dataSource := BuildDataSource(config, false, nil)
			spinner.Start()

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

			var foundKubeconfig *string = nil
			for i, a := range args {
				if strings.HasPrefix(a, "--") {
					if len(args) < i+2 || strings.HasPrefix(args[i+1], "--") {
						return fmt.Errorf("%s has no value", args[i])
					}
					// since automatic flag parsing is disabled to allow dynamic flags
					// flags need to be parsed manually here eg. kubeconfig
					if a == "--kubeconfig" {
						foundKubeconfig = &args[i+1]
					}
				}
			}

			//dynamic flags parsing
			for k, _ := range collectedFlags {
				field := scenarioDetail.GetFieldByName(k)
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
					if value != nil {
						environment[*field.Variable] = *value
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
			kubeconfigPath, err := container_manager.PrepareKubeconfig(foundKubeconfig, config)
			if err != nil {
				return err
			}
			//WIP
			socket, err := (*containerManager).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}

			containerId, err := (*containerManager).RunAndStream(config.GetQuayImageUri()+":"+scenarioDetail.Name,
				scenarioDetail.Name,
				*socket,
				environment,
				false,
				map[string]string{},
				*kubeconfigPath,
				scenarioDetail.KubeconfigPath,
			)
			if err != nil {
				return err
			}
			fmt.Println(fmt.Sprintf("container started: %s", *containerId))
			return nil
		},
	}
	return runCmd
}
