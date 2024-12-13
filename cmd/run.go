package cmd

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"log"
	"os"
	"strings"
	"time"
)

func NewRunCommand(factory *factory.ProviderFactory, scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	scenarioCollectedFlags := make(map[string]*string)
	globalCollectedFlags := make(map[string]*string)
	var command = &cobra.Command{
		Use:                "run",
		Short:              "runs a scenario",
		Long:               `runs a scenario`,
		DisableFlagParsing: false,
		Args:               cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			provider := GetProvider(false, factory)
			scenarios, err := FetchScenarios(provider)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			provider := GetProvider(false, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0])
			if err != nil {
				return err
			}
			globalEnvDetail, err := provider.GetGlobalEnvironment()
			if err != nil {
				return err
			}

			globalFlags := pflag.NewFlagSet("global", pflag.ExitOnError)
			scenarioFlags := pflag.NewFlagSet("scenario", pflag.ExitOnError)

			if scenarioDetail == nil {
				return fmt.Errorf("%s scenario not found", args[0])
			}

			for _, field := range scenarioDetail.Fields {
				var defaultValue = ""
				if field.Default != nil {
					defaultValue = *field.Default
				}
				scenarioCollectedFlags[*field.Name] = scenarioFlags.String(*field.Name, defaultValue, *field.Description)
				if err != nil {
					return err
				}

			}

			for _, field := range globalEnvDetail.Fields {
				var defaultValue = ""
				if field.Default != nil {
					defaultValue = *field.Default
				}
				globalCollectedFlags[*field.Name] = globalFlags.String(*field.Name, defaultValue, *field.Description)
				if err != nil {
					return err
				}
			}

			//cmd.LocalFlags().AddFlagSet(globalFlags)
			cmd.LocalFlags().AddFlagSet(scenarioFlags)

			cmd.SetHelpFunc(func(command *cobra.Command, args []string) {
				yellow := color.New(color.FgYellow).SprintFunc()
				green := color.New(color.FgGreen).SprintFunc()
				boldGreen := color.New(color.FgHiGreen, color.Bold).SprintFunc()

				fmt.Println(fmt.Sprintf("%s", yellow("Krkn Global flags")))
				for _, f := range globalEnvDetail.Fields {
					if f.Default != nil && *f.Default != "" {
						fmt.Printf("  --%s (%s): %s (Default: %s)\n", *f.Name, f.Type.String(), *f.Description, *f.Default)
					} else {
						fmt.Printf("  --%s (%s): %s\n", *f.Name, f.Type.String(), *f.Description)
					}

				}

				fmt.Println(fmt.Sprintf("\n%s %s", boldGreen(scenarioDetail.Name), green("Flags")))
				for _, f := range scenarioDetail.Fields {
					if f.Default != nil && *f.Default != "" {
						fmt.Printf("  --%s (%s): %s (Default: %s)\n", *f.Name, f.Type.String(), *f.Description, *f.Default)
					} else {
						fmt.Printf("  --%s (%s): %s\n", *f.Name, f.Type.String(), *f.Description)
					}

				}

				fmt.Print("\n\n")

			})

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			(*scenarioOrchestrator).PrintContainerRuntime()
			spinner := NewSpinnerWithSuffix("fetching scenario metadata...")

			// Starts validating input message
			spinner.Start()

			runDetached := false
			debug := false

			provider := GetProvider(false, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0])
			if err != nil {
				return err
			}
			globalDetail, err := provider.GetGlobalEnvironment()
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
					// flags need to be parsed manually here e.g. kubeconfig
					if a == "--kubeconfig" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						if CheckFileExists(args[i+1]) == false {
							return fmt.Errorf("file %s does not exist", args[i+1])
						}
						foundKubeconfig = &args[i+1]
					}
					if a == "--alerts-profile" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						if CheckFileExists(args[i+1]) == false {
							return fmt.Errorf("file %s does not exist", args[i+1])
						}
						alertsProfile = &args[i+1]
					}
					if a == "--metrics-profile" {
						if err := checkStringArgValue(args, i); err != nil {
							return err
						}
						if CheckFileExists(args[i+1]) == false {
							return fmt.Errorf("file %s does not exist", args[i+1])
						}
						metricsProfile = &args[i+1]
					}

					if a == "--detached" {
						runDetached = true
					}

					if a == "--debug" {
						debug = true
					}

					if a == "--help" {
						spinner.Stop()
						cmd.Help()
						return nil
					}
				}
			}

			kubeconfigPath, err := utils.PrepareKubeconfig(foundKubeconfig, config)
			if err != nil {
				return err
			}
			if kubeconfigPath == nil {
				return fmt.Errorf("kubeconfig not found: %s", *foundKubeconfig)
			}
			volumes[*kubeconfigPath] = config.KubeconfigPath
			if metricsProfile != nil {
				volumes[*metricsProfile] = config.MetricsProfilePath
			}

			if alertsProfile != nil {
				volumes[*alertsProfile] = config.AlertsProfilePath
			}
			spinner.Suffix = "validating input ..."
			//dynamic flags parsing
			scenarioEnv, scenarioVol, err := ParseFlags(scenarioDetail, args, scenarioCollectedFlags, false)
			if err != nil {
				return err
			}
			globalEnv, globalVol, err := ParseFlags(globalDetail, args, globalCollectedFlags, true)
			if err != nil {
				return err
			}

			for k, v := range *scenarioVol {
				volumes[k] = v
			}

			for k, v := range *globalVol {
				volumes[k] = v
			}

			for k, v := range *scenarioEnv {
				environment[k] = v
			}
			for k, v := range *globalEnv {
				environment[k] = v
			}

			spinner.Stop()

			tbl := NewEnvironmentTable(environment)
			tbl.Print()
			fmt.Print("\n")
			// restarts the spinner to present image pull progress
			spinner.Suffix = "pulling scenario image..."
			spinner.Start()

			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			conn, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}
			startTime := time.Now()
			containerName := utils.GenerateContainerName(config, scenarioDetail.Name, nil)
			quayImageUri, err := config.GetQuayImageUri()
			if err != nil {
				return err
			}

			if runDetached == false {
				commChan := make(chan *string)
				go func() {
					for msg := range commChan {
						spinner.Suffix = *msg
					}
					spinner.Stop()
				}()

				_, err = (*scenarioOrchestrator).RunAttached(quayImageUri+":"+scenarioDetail.Name, containerName, environment, false, volumes, os.Stdout, os.Stderr, &commChan, conn, debug)
				if err != nil {
					var staterr *utils.ExitError
					if errors.As(err, &staterr) {
						os.Exit(staterr.ExitStatus)
					}
					return err
				}
				scenarioDuration := time.Since(startTime)
				fmt.Println(fmt.Sprintf("%s ran for %s", scenarioDetail.Name, scenarioDuration.String()))
			} else {
				containerId, err := (*scenarioOrchestrator).Run(quayImageUri+":"+scenarioDetail.Name, containerName, environment, false, volumes, nil, conn, debug)
				if err != nil {
					return err
				}
				spinner.Stop()
				_, err = color.New(color.FgGreen, color.Underline).Println(fmt.Sprintf("scenario %s started with containerId %s", scenarioDetail.Name, *containerId))
				if err != nil {
					return err
				}
			}

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
