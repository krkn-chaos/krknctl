package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/randomgraph"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
	"time"
)

func NewRandomCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "random",
		Short: "runs or scaffolds a random chaos run based on a json test plan",
		Long:  `runs or scaffolds a random chaos run based on a json test plan`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return command
}

func NewRandomRunCommand(factory *providerfactory.ProviderFactory, scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "run",
		Short: "runs a random chaos run",
		Long:  `runs a random run based on a json test plan`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registrySettings, err := providermodels.NewRegistryV2FromEnv(config)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return err
				}
			}
			(*scenarioOrchestrator).PrintContainerRuntime()
			if registrySettings != nil {
				logPrivateRegistry(registrySettings.RegistryURL)
			}
			spinner := NewSpinnerWithSuffix("running randomly generated chaos plan...")
			volumes := make(map[string]string)
			environment := make(map[string]string)
			kubeconfig, err := cmd.Flags().GetString("kubeconfig")
			if err != nil {
				return err
			}
			if kubeconfig != "" {
				expandedConfig, err := commonutils.ExpandFolder(kubeconfig, nil)
				if err != nil {
					return err
				}
				kubeconfig = *expandedConfig
				if !CheckFileExists(kubeconfig) {
					return fmt.Errorf("file %s does not exist", kubeconfig)
				}
			}
			alertsProfile, err := cmd.Flags().GetString("alerts-profile")
			if err != nil {
				return err
			}
			if alertsProfile != "" {
				expandedProfile, err := commonutils.ExpandFolder(alertsProfile, nil)
				if err != nil {
					return err
				}
				alertsProfile = *expandedProfile
				if !CheckFileExists(alertsProfile) {
					return fmt.Errorf("file %s does not exist", alertsProfile)
				}
			}
			metricsProfile, err := cmd.Flags().GetString("metrics-profile")
			if err != nil {
				return err
			}
			if metricsProfile != "" {
				expandedProfile, err := commonutils.ExpandFolder(metricsProfile, nil)
				if err != nil {
					return err
				}
				metricsProfile = *expandedProfile
				if !CheckFileExists(metricsProfile) {
					return fmt.Errorf("file %s does not exist", metricsProfile)
				}
			}
			maxParallel, err := cmd.Flags().GetInt64("max-parallel")
			if err != nil {
				return err
			}

			numberOfScenarios, err := cmd.Flags().GetInt("number-of-scenarios")
			if err != nil {
				return err
			}

			exitOnerror, err := cmd.Flags().GetBool("exit-on-error")
			if err != nil {
				return err
			}

			randomGraphFile, err := cmd.Flags().GetString("graph-dump")
			if err != nil {
				return err
			}

			if randomGraphFile != "" {
				randomGraphFile = fmt.Sprintf(config.RandomGraphPath, time.Now().Unix())
			}

			kubeconfigPath, err := utils.PrepareKubeconfig(&kubeconfig, config)
			if err != nil {
				return err
			}
			if kubeconfigPath == nil {
				return fmt.Errorf("kubeconfig not found: %s", kubeconfig)
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

			if err != nil {
				return err
			}
			privateRegistry := false
			if registrySettings != nil {
				privateRegistry = true
			}
			dataProvider := GetProvider(privateRegistry, factory)

			nameChannel := make(chan *struct {
				name *string
				err  error
			})
			spinner.Start()
			go func() {
				validateGraphScenarioInput(dataProvider, nodes, nameChannel, registrySettings)
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

			executionPlan := randomgraph.NewRandomGraph(nodes, maxParallel, numberOfScenarios)
			if err = DumpRandomGraph(nodes, executionPlan, randomGraphFile, config.LabelRootNode); err != nil {
				return err
			}
			if len(executionPlan) == 0 {
				_, err = color.New(color.FgYellow).Println("No scenario to execute; the random graph file appears to be empty (single-node graphs are not supported).")
				if err != nil {
					return err
				}
				return nil
			}

			table, err := NewGraphTable(executionPlan, config)
			if err != nil {
				return err
			}
			table.Print()
			fmt.Print("\n\n")
			spinner.Suffix = "starting chaos scenarios..."
			spinner.Start()

			commChannel := make(chan *models.GraphCommChannel)

			go func() {
				(*scenarioOrchestrator).RunGraph(nodes, executionPlan, environment, volumes, false, commChannel, registrySettings, nil)
			}()

			for {
				c := <-commChannel
				if c == nil {
					break
				} else {
					if c.Err != nil {
						spinner.Stop()
						var statErr *utils.ExitError
						if errors.As(c.Err, &statErr) {
							if c.ScenarioID != nil && c.ScenarioLogFile != nil {
								_, err = color.New(color.FgHiRed).Println(fmt.Sprintf("scenario %s at step %d with exit status %d, check log file %s aborting chaos run.",
									*c.ScenarioID,
									*c.Layer,
									statErr.ExitStatus,
									*c.ScenarioLogFile))
								if err != nil {
									return err
								}
							}
							if exitOnerror {
								_, err = color.New(color.FgHiRed).Println(fmt.Sprintf("aborting chaos run with exit status %d", statErr.ExitStatus))
								if err != nil {
									return err
								}
								os.Exit(statErr.ExitStatus)
							}
							spinner.Start()
						}

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

func NewRandomScaffoldCommand(factory *providerfactory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "scaffold",
		Short: "scaffolds a random chaos run",
		Long:  `scaffolds a random run based on a json test plan`,
		Args:  cobra.MinimumNArgs(0),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			registrySettings, err := providermodels.NewRegistryV2FromEnv(config)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
			}
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return nil, cobra.ShellCompDirectiveError
			}
			dataProvider := GetProvider(registrySettings != nil, factory)
			scenarios, err := FetchScenarios(dataProvider, registrySettings)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var seed *provider.ScaffoldSeed
			registrySettings, err := providermodels.NewRegistryV2FromEnv(config)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return err
				}
			}
			dataProvider := GetProvider(registrySettings != nil, factory)
			includeGlobalEnv, err := cmd.Flags().GetBool("global-env")
			if err != nil {
				return err
			}
			seedFile, err := cmd.Flags().GetString("seed-file")
			if err != nil {
				return err
			}
			numberOfScenarios, err := cmd.Flags().GetInt("number-of-scenarios")
			if err != nil {
				return err
			}

			if seedFile != "" {
				seedFilePath, err := commonutils.ExpandFolder(seedFile, nil)
				if err != nil {
					return err
				}
				if !CheckFileExists(*seedFilePath) {
					return fmt.Errorf("file %s does not exist", seedFile)
				}
				seed = &provider.ScaffoldSeed{
					NumberOfScenarios: numberOfScenarios,
					Path:              *seedFilePath,
				}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("please provide at least one scenario")
				}
			}

			output, err := dataProvider.ScaffoldScenarios(args, includeGlobalEnv, registrySettings, true, seed)
			if err != nil {
				return err
			}
			fmt.Println(*output)
			return nil
		},
	}
	return command
}
