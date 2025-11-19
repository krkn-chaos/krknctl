package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/resiliency"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// 🤖 Assisted with Claude Code (claude.ai/code)
// Fixed flag parsing to support both --flag=value and --flag value formats

func NewRunCommand(factory *factory.ProviderFactory, scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	scenarioCollectedFlags := make(map[string]*string)
	globalCollectedFlags := make(map[string]*string)
	var command = &cobra.Command{
		Use:                "run",
		Short:              "runs a scenario",
		Long:               `runs a scenario`,
		DisableFlagParsing: false,
		Args:               cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			registrySettings, err := parsePrivateRepoArgs(cmd, &args)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return nil, cobra.ShellCompDirectiveError
			}
			provider := GetProvider(registrySettings != nil, factory)
			scenarios, err := FetchScenarios(provider, registrySettings)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			registrySettings, err := models.NewRegistryV2FromEnv(config)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, &args)
				if err != nil {
					return err
				}
			}

			scenarioName, err := parseScenarioName(args)
			if err != nil {
				return err
			}

			provider := GetProvider(registrySettings != nil, factory)

			scenarioDetail, err := provider.GetScenarioDetail(scenarioName, registrySettings)
			if err != nil {
				return err
			}
			globalEnvDetail, err := provider.GetGlobalEnvironment(registrySettings, scenarioName)
			if err != nil {
				return err
			}

			globalFlags := pflag.NewFlagSet("global", pflag.ExitOnError)
			scenarioFlags := pflag.NewFlagSet("scenario", pflag.ExitOnError)

			if scenarioDetail == nil {
				return fmt.Errorf("%s scenario not found", scenarioName)
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

			cmd.LocalFlags().AddFlagSet(globalFlags)
			cmd.LocalFlags().AddFlagSet(scenarioFlags)

			cmd.SetHelpFunc(func(command *cobra.Command, args []string) {
				yellow := color.New(color.FgYellow).SprintFunc()
				green := color.New(color.FgGreen).SprintFunc()
				boldGreen := color.New(color.FgHiGreen, color.Bold).SprintFunc()
				fmt.Printf("%s", yellow("Krkn Global flags\n"))
				printHelp(*globalEnvDetail)
				fmt.Printf("\n%s %s\n", boldGreen(scenarioDetail.Name), green("Flags"))
				printHelp(*scenarioDetail)
				fmt.Print("\n\n")
			})

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			registrySettings, err := models.NewRegistryV2FromEnv(config)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, &args)
				if err != nil {
					return err
				}
			}
			if registrySettings != nil {
				logPrivateRegistry(registrySettings.RegistryURL)
			}

			if err != nil {
				return err
			}

			scenarioName, err := parseScenarioName(args)
			if err != nil {
				return err
			}

			(*scenarioOrchestrator).PrintContainerRuntime()
			spinner := NewSpinnerWithSuffix("fetching scenario metadata...")
			spinner.Start()

			runDetached := false

			provider := GetProvider(registrySettings != nil, factory)
			scenarioDetail, err := provider.GetScenarioDetail(scenarioName, registrySettings)
			if err != nil {
				spinner.Stop()
				return err
			}
			globalDetail, err := provider.GetGlobalEnvironment(registrySettings, scenarioName)
			if err != nil {
				spinner.Stop()
				return err
			}

			environment := make(map[string]string)
			parsedFields := make(map[string]ParsedField)
			volumes := make(map[string]string)
			var foundKubeconfig *string = nil
			var foundAlertsProfile *string = nil
			var foundMetricsProfile *string = nil
			// since automatic flag parsing is disabled to allow dynamic flags
			// flags need to be parsed manually here e.g. kubeconfig
			if value, found, err := ParseArgValue(args, "--kubeconfig"); err != nil {
				return err
			} else if found {
				expandedConfig, err := commonutils.ExpandFolder(value, nil)
				if err != nil {
					spinner.Stop()
					return err
				}
				if !CheckFileExists(*expandedConfig) {
					return fmt.Errorf("file %s does not exist", value)
				}
				foundKubeconfig = expandedConfig
			}

			if value, found, err := ParseArgValue(args, "--alerts-profile"); err != nil {
				spinner.Stop()
				return err
			} else if found {
				expandedProfile, err := commonutils.ExpandFolder(value, nil)
				if err != nil {
					spinner.Stop()
					return err
				}
				if !CheckFileExists(*expandedProfile) {
					return fmt.Errorf("file %s does not exist", *expandedProfile)
				}
				foundAlertsProfile = expandedProfile
			}

			if value, found, err := ParseArgValue(args, "--metrics-profile"); err != nil {
				spinner.Stop()
				return err
			} else if found {
				expandedProfile, err := commonutils.ExpandFolder(value, nil)
				if err != nil {
					spinner.Stop()
					return err
				}
				if !CheckFileExists(*expandedProfile) {
					spinner.Stop()
					return fmt.Errorf("file %s does not exist", *expandedProfile)
				}
				foundMetricsProfile = expandedProfile
			}

			// Handle boolean flags
			for _, a := range args {
				if a == "--detached" {
					runDetached = true
				}
				if a == "--help" {
					spinner.Stop()
					if err := cmd.Help(); err != nil {
						return err
					}
					return nil
				}
			}

			kubeconfigPath, err := utils.PrepareKubeconfig(foundKubeconfig, config)
			if err != nil {
				spinner.Stop()
				return err
			}
			if kubeconfigPath == nil {
				spinner.Stop()
				return errors.New("kubeconfig not found on default path, please specify a " +
					"valid kubeconfig path with the --kubeconfig flag")
			}
			volumes[*kubeconfigPath] = config.KubeconfigPath
			if foundMetricsProfile != nil {
				volumes[*foundMetricsProfile] = config.MetricsProfilePath
			}

			if foundAlertsProfile != nil {
				volumes[*foundAlertsProfile] = config.AlertsProfilePath
			}
			spinner.Suffix = "validating input ..."
			//dynamic flags parsing
			scenarioEnv, scenarioVol, err := ParseFlags(scenarioDetail, args, scenarioCollectedFlags, false)
			if err != nil {
				spinner.Stop()
				return err
			}
			globalEnv, globalVol, err := ParseFlags(globalDetail, args, globalCollectedFlags, true)
			if err != nil {
				spinner.Stop()
				return err
			}

			for k, v := range *scenarioVol {
				volumes[k] = v
			}

			for k, v := range *globalVol {
				volumes[k] = v
			}

			for k, v := range *scenarioEnv {
				parsedFields[k] = v
				environment[k] = v.value
			}
			for k, v := range *globalEnv {
				parsedFields[k] = v
				environment[k] = v.value
			}

			spinner.Stop()

			tbl := NewEnvironmentTable(parsedFields, config)
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
			quayImageURI, err := config.GetCustomDomainImageURI()
			if err != nil {
				return err
			}

			if !runDetached {
				// capture stdout/stderr to both terminal and a log file for parsing
				filename := fmt.Sprintf("%s.log", containerName)
				file, cerr := os.Create(path.Clean(filename))
				if cerr != nil {
					spinner.Stop()
					return cerr
				}
				defer file.Close()
				mw := io.MultiWriter(os.Stdout, file)

				commChan := make(chan *string)
				go func() {
					for msg := range commChan {
						spinner.Suffix = *msg
					}
					spinner.Stop()
				}()

				_, err = (*scenarioOrchestrator).RunAttached(quayImageURI+":"+scenarioDetail.Name, containerName, environment, false, volumes, mw, mw, &commChan, conn, registrySettings)
				if err != nil {
					var staterr *utils.ExitError
					if errors.As(err, &staterr) {
						os.Exit(staterr.ExitStatus)
					}
					return err
				}

				_ = file.Sync()

				// Parse resiliency report from captured logs and generate report
				if data, rerr := os.ReadFile(filename); rerr == nil {
					if rep, perr := resiliency.ParseResiliencyReport(data); perr == nil {
						if err := resiliency.GenerateAndWriteReport(
							[]resiliency.DetailedScenarioReport{*rep},
							"resiliency-report.json",
						); err != nil {
							log.Printf("Error generating resiliency report: %v", err)
						}
					}
				}

				scenarioDuration := time.Since(startTime)
				fmt.Printf("%s ran for %s\n", scenarioDetail.Name, scenarioDuration.String())
			} else {
				containerID, err := (*scenarioOrchestrator).Run(quayImageURI+":"+scenarioDetail.Name, containerName, environment, false, volumes, nil, conn, registrySettings)
				if err != nil {
					return err
				}
				spinner.Stop()
				_, err = color.New(color.FgGreen, color.Underline).Println(fmt.Sprintf("scenario %s started with containerID %s", scenarioDetail.Name, *containerID))
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	return command
}

func printHelp(scenario models.ScenarioDetail) {
	boldWhite := color.New(color.FgHiWhite, color.Bold).SprintFunc()
	for _, f := range scenario.Fields {
		
		enum := ""
		if f.Type == typing.Enum {
			enum = strings.Replace(*f.AllowedValues, *f.Separator, "|", -1)
		}
		def := ""
		if f.Default != nil && *f.Default != "" {
			def = fmt.Sprintf("(Default: %s)", *f.Default)
		}

		fmt.Printf("\t--%s %s: %s [%s]%s\n", *f.Name, boldWhite(enum), *f.Description, boldWhite(f.Type.String()), def)
	}
}

func parseScenarioName(args []string) (string, error) {
	if !strings.HasPrefix(args[0], "--") {
		return args[0], nil
	}

	if !strings.HasPrefix(args[len(args)-1], "--") {
		return args[len(args)-1], nil
	}

	if len(args)-2 >= 0 {
		return args[len(args)-2], nil
	}

	return "", errors.New("please enter a scenario name")

}
