package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/dependencygraph"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
)

func NewGraphCommand() *cobra.Command {
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

func NewGraphRunCommand(factory *providerfactory.ProviderFactory, scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "run",
		Short: "Runs a dependency graph based run",
		Long:  `Runs graph based run`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			(*scenarioOrchestrator).PrintContainerRuntime()
			spinner := NewSpinnerWithSuffix("running graph based chaos plan...")
			volumes := make(map[string]string)
			environment := make(map[string]string)
			kubeconfig, err := cmd.Flags().GetString("kubeconfig")
			if err != nil {
				return err
			}
			if CheckFileExists(kubeconfig) == false {
				return fmt.Errorf("file %s does not exist", kubeconfig)
			}
			alertsProfile, err := cmd.Flags().GetString("alerts-profile")
			if err != nil {
				return err
			}
			if alertsProfile != "" && CheckFileExists(alertsProfile) == false {
				return fmt.Errorf("file %s does not exist", alertsProfile)
			}
			metricsProfile, err := cmd.Flags().GetString("metrics-profile")
			if err != nil {
				return err
			}
			if metricsProfile != "" && CheckFileExists(metricsProfile) == false {
				return fmt.Errorf("file %s does not exist", metricsProfile)
			}

			kubeconfigPath, err := utils.PrepareKubeconfig(&kubeconfig, config)
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

			nodes := make(map[string]models.ScenarioNode)
			err = json.Unmarshal(file, &nodes)

			dataSource, err := BuildDataSource(config, false, nil)
			if err != nil {
				return err
			}
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

			commChannel := make(chan *models.GraphCommChannel)
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}

			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}

			go func() {
				(*scenarioOrchestrator).RunGraph(nodes, executionPlan, environment, volumes, false, commChannel, ctx)
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
					spinner.Suffix = fmt.Sprintf("Running step %d scenario(s): %s", *c.Layer, strings.Join(executionPlan[*c.Layer], ", "))

				}

			}
			spinner.Stop()

			return nil
		},
	}
	return command
}

func validateScenariosInput(provider provider.ScenarioDataProvider, dataSource string, nodes map[string]models.ScenarioNode, scenarioNameChannel chan *struct {
	name *string
	err  error
}) {
	for _, n := range nodes {
		// skip _comment
		if n.Name == "" {
			continue
		}
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
		if scenarioDetail == nil {
			scenarioNameChannel <- &struct {
				name *string
				err  error
			}{name: &n.Name, err: fmt.Errorf("scenario %s not found in %s", n.Name, dataSource)}
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

func NewGraphScaffoldCommand(factory *providerfactory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "scaffold",
		Short: "Scaffolds a dependency graph based run",
		Long:  `Scaffolds a dependency graph based run`,
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			dataSource, err := BuildDataSource(config, false, nil)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			dataProvider := GetProvider(false, factory)

			scenarios, err := FetchScenarios(dataProvider, dataSource)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dataSource, err := BuildDataSource(config, false, nil)
			if err != nil {
				return err
			}
			dataProvider := GetProvider(false, factory)
			output, err := dataProvider.ScaffoldScenarios(args, dataSource)
			if err != nil {
				return err
			}
			fmt.Println(*output)
			return nil
		},
	}
	return command
}
