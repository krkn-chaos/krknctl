package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	provider_factory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"log"
	"os"
)

func NewGraphCommand(factory *provider_factory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "graph",
		Short: "Runs or scaffolds a dependency graph based run",
		Long:  `Runs or scaffolds a dependency graph based run`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return command
}

func NewGraphRunCommand(factory *provider_factory.ProviderFactory, containerManager *container_manager.ContainerManager, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "run",
		Short: "Runs a dependency graph based run",
		Long:  `Runs graph based run`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spinner := NewSpinnerWithSuffix("running graph based chaos plan...")
			container_manager.PrintDetectedContainerRuntime(containerManager)
			volumes := make(map[string]string)
			environment := make(map[string]string)
			kubeconfig, err := cmd.LocalFlags().GetString("kubeconfig")
			if err != nil {
				return err
			}
			alertsProfile, err := cmd.LocalFlags().GetString("alerts-profile")
			if err != nil {
				return err
			}
			metricsProfile, err := cmd.LocalFlags().GetString("metrics-profile")
			if err != nil {
				return err
			}

			kubeconfigPath, err := container_manager.PrepareKubeconfig(&kubeconfig, config)
			if err != nil {
				return err
			}
			volumes[*kubeconfigPath] = config.KubeconfigPath

			if metricsProfile != "" {
				volumes[metricsProfile] = config.MetricsProfilePath
			}

			if alertsProfile != "" {
				volumes[alertsProfile] = config.AlertsProfilePath
			}

			file, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to open scenario file: %s", args[0])
			}

			nodes := make(map[string]container_manager.ScenarioNode)
			err = json.Unmarshal(file, &nodes)

			dataSource := BuildDataSource(config, false, nil)
			dataProvider := GetProvider(false, factory)
			nameChannel := make(chan *struct {
				name *string
				err  error
			})
			spinner.Start()
			go func() {
				validateScenariosInput(dataProvider, dataSource, nodes, nameChannel)
			}()

			for {
				validateResult := <-nameChannel
				if validateResult == nil {
					break
				}
				if validateResult.err != nil {
					return fmt.Errorf("failed to validate scenario: %s, error: %s", *validateResult.name, validateResult.err)
				}
				if validateResult.name != nil {
					spinner.Suffix = fmt.Sprintf("validating input for scenario: %s", *validateResult.name)
				}
			}

			spinner.Stop()

			convertedNodes := make(map[string]dependencygraph.ParentProvider, len(nodes))

			// Populate the new map
			for key, node := range nodes {
				// Since ScenarioNode implements ParentProvider, this is valid
				convertedNodes[key] = node
			}
			graph, err := dependencygraph.NewGraphFromNodes(convertedNodes)
			if err != nil {
				return err
			}

			executionPlan := graph.TopoSortedLayers()
			table := NewGraphTable(executionPlan)
			table.Print()
			fmt.Print("\n\n")
			spinner.Suffix = "starting chaos scenarios..."
			spinner.Start()

			commChannel := make(chan *container_manager.CommChannel)
			socket, err := (*containerManager).GetContainerRuntimeSocket(nil)

			if err != nil {
				return err
			}
			go func() {
				(*containerManager).RunGraph(nodes, executionPlan, *socket, environment, volumes, false, commChannel)
			}()

			for {
				c := <-commChannel
				if c == nil {
					break
				} else {
					if c.Err != nil {
						// interrupt all running scenarios
						return c.Err
					}
					spinner.FinalMSG = fmt.Sprintf("Running step %d scenario: %s\n", *c.Layer, *c.ScenarioId)

				}

			}
			spinner.Stop()

			return nil
		},
	}
	return command
}

func validateScenariosInput(provider provider.ScenarioDataProvider, dataSource string, nodes map[string]container_manager.ScenarioNode, scenarioNameChannel chan *struct {
	name *string
	err  error
}) {
	for _, n := range nodes {
		scenarioNameChannel <- &struct {
			name *string
			err  error
		}{name: &n.Name, err: nil}
		scenarioDetail, err := provider.GetScenarioDetail(n.Name, dataSource)
		if err != nil {
			scenarioNameChannel <- &struct {
				name *string
				err  error
			}{name: &n.Name, err: err}
			return
		}
		for k, v := range n.Env {
			field := scenarioDetail.GetFieldByEnvVar(k)
			if field == nil {

				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: fmt.Errorf("environment variable %s not found", k)}
				return
			}
			_, err := field.Validate(&v)
			if err != nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: err}
				return
			}
		}

		for k, v := range n.Volumes {
			field := scenarioDetail.GetFileFieldByMountPath(v)
			if field == nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: fmt.Errorf("no file parameter found of type field for scenario %s with mountPath %s", n.Name, v)}
				return
			}
			_, err := field.Validate(&k)
			if err != nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: err}
				return
			}
		}
	}
	scenarioNameChannel <- nil
}

func NewGraphScaffoldCommand(factory *provider_factory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "scaffold",
		Short: "Scaffolds a dependency graph based run",
		Long:  `Scaffolds a dependency graph based run`,
		Args:  cobra.MinimumNArgs(1),
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
		RunE: func(cmd *cobra.Command, args []string) error {
			dataSource := BuildDataSource(config, false, nil)
			provider := GetProvider(false, factory)
			output, err := provider.ScaffoldScenarios(args, dataSource)
			if err != nil {
				return err
			}
			fmt.Println(*output)
			return nil
		},
	}
	return command
}