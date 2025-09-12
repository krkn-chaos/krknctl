// Package cmd contains the CLI options and commands to interact with krknctl
// Assisted by Claude Sonnet 4
package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/spf13/cobra"
	"os"
)

func Execute(providerFactory *factory.ProviderFactory, scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) {

	rootCmd := NewRootCommand(config)
	rootCmd.PersistentFlags().String("private-registry", "", "private registry URI (eg. quay.io, without any protocol schema prefix)")
	rootCmd.PersistentFlags().String("private-registry-username", "", "private registry username for basic authentication")
	rootCmd.PersistentFlags().String("private-registry-password", "", "private registry password for basic authentication")
	rootCmd.PersistentFlags().Bool("private-registry-insecure", false, "uses plain HTTP instead of TLS")
	rootCmd.PersistentFlags().Bool("private-registry-skip-tls", false, "skips tls verification on private registry")
	rootCmd.PersistentFlags().String("private-registry-token", "", "private registry identity token for token based authentication")
	rootCmd.PersistentFlags().String("private-registry-scenarios", "", "private registry krkn scenarios image repository")
	rootCmd.PersistentFlags().String("private-registry-lightspeed", "", "private registry lightspeed image repository")
	var completionCmd = &cobra.Command{
		Use:       "completion [bash|zsh]",
		Short:     "Genera script di completamento per bash o zsh",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				err := rootCmd.GenBashCompletion(os.Stdout)
				if err != nil {
					return err
				}
			case "zsh":
				err := rootCmd.GenZshCompletion(os.Stdout)
				if err != nil {
					return err
				}
			default:
				fmt.Println("shell not supported:", args[0])
			}
			return nil
		},
	}

	rootCmd.AddCommand(completionCmd)

	listCmd := NewListCommand()
	listScenariosCmd := NewListScenariosCommand(providerFactory, config)
	listRunningCmd := NewListRunningScenario(scenarioOrchestrator)
	listCmd.AddCommand(listScenariosCmd)
	listCmd.AddCommand(listRunningCmd)
	rootCmd.AddCommand(listCmd)

	describeCmd := NewDescribeCommand(providerFactory, config)
	rootCmd.AddCommand(describeCmd)

	runCmd := NewRunCommand(providerFactory, scenarioOrchestrator, config)
	runCmd.LocalFlags().String("kubeconfig", "", "kubeconfig path (if not set will default to ~/.kube/config)")
	runCmd.LocalFlags().String("alerts-profile", "", "custom alerts profile file path")
	runCmd.LocalFlags().String("metrics-profile", "", "custom metrics profile file path")
	runCmd.LocalFlags().Bool("detached", false, "if set this flag will run in detached mode")
	runCmd.LocalFlags().Bool("form", false, "Use interactive form to collect scenario parameters instead of CLI flags")
	runCmd.DisableFlagParsing = true
	rootCmd.AddCommand(runCmd)

	cleanCmd := NewCleanCommand(scenarioOrchestrator, config)
	rootCmd.AddCommand(cleanCmd)

	// graph subcommands
	graphCmd := NewGraphCommand()
	graphRunCmd := NewGraphRunCommand(providerFactory, scenarioOrchestrator, config)
	graphRunCmd.Flags().String("kubeconfig", "", "kubeconfig path (if not set will default to ~/.kube/config)")
	graphRunCmd.Flags().String("alerts-profile", "", "custom alerts profile file path")
	graphRunCmd.Flags().String("metrics-profile", "", "custom metrics profile file path")
	graphRunCmd.Flags().Bool("exit-on-error", false, "if set this flag will the workflow will be interrupted and the tool will exit with a status greater than 0")
	graphScaffoldCmd := NewGraphScaffoldCommand(providerFactory, config)
	graphScaffoldCmd.Flags().Bool("global-env", false, "if set this flag will add global environment variables to each scenario in the graph")
	graphCmd.AddCommand(graphRunCmd)
	graphCmd.AddCommand(graphScaffoldCmd)
	rootCmd.AddCommand(graphCmd)

	// random subcommand
	randomCmd := NewRandomCommand()
	randomRunCmd := NewRandomRunCommand(providerFactory, scenarioOrchestrator, config)
	randomRunCmd.Flags().String("kubeconfig", "", "kubeconfig path (if not set will default to ~/.kube/config)")
	randomRunCmd.Flags().String("alerts-profile", "", "custom alerts profile file path")
	randomRunCmd.Flags().String("metrics-profile", "", "custom metrics profile file path")
	randomRunCmd.Flags().Int("max-parallel", 0, "maximum number of parallel scenarios")
	randomRunCmd.Flags().Int("number-of-scenarios", 0, "allows you to specify the number of elements to select from the execution plan")
	randomRunCmd.Flags().Bool("exit-on-error", false, "if set this flag will the workflow will be interrupted and the tool will exit with a status greater than 0")
	randomRunCmd.Flags().String("graph-dump", "", "specifies the name of the file where the randomly generated dependency graph will be persisted")
	err := randomRunCmd.MarkFlagRequired("max-parallel")
	if err != nil {
		fmt.Println("Error marking flag as required:", err)
		os.Exit(1)
	}

	randomScaffoldCmd := NewRandomScaffoldCommand(providerFactory, config)
	randomScaffoldCmd.Flags().Bool("global-env", false, "if set this flag will add global environment variables to each scenario in the graph")
	randomScaffoldCmd.Flags().String("seed-file", "", "template file with already configured scenarios used to generate the random test plan")
	randomScaffoldCmd.Flags().Int("number-of-scenarios", 0, "the number of scenarios that will be created from the template file")
	randomScaffoldCmd.MarkFlagsRequiredTogether("seed-file", "number-of-scenarios")
	randomCmd.AddCommand(randomRunCmd)
	randomCmd.AddCommand(randomScaffoldCmd)
	rootCmd.AddCommand(randomCmd)

	attachCmd := NewAttachCmd(scenarioOrchestrator)
	rootCmd.AddCommand(attachCmd)
	queryCmd := NewQueryStatusCommand(scenarioOrchestrator)
	queryCmd.Flags().String("graph", "", "to query the exit status of a previously run graph file")
	rootCmd.AddCommand(queryCmd)

	// lightspeed subcommands
	lightspeedCmd := NewLightspeedCommand()
	lightspeedCheckCmd := NewLightspeedCheckCommand(providerFactory, scenarioOrchestrator, config)
	lightspeedRunCmd := NewLightspeedRunCommand(providerFactory, scenarioOrchestrator, config)
	lightspeedCmd.AddCommand(lightspeedCheckCmd)
	lightspeedCmd.AddCommand(lightspeedRunCmd)
	rootCmd.AddCommand(lightspeedCmd)

	// update and deprecation check
	isDeprecated, err := IsDeprecated(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if isDeprecated != nil && *isDeprecated {
		_, err = color.New(color.FgHiRed).Println(fmt.Sprintf("⛔️ krknctl %s is deprecated, please update to latest: %s", config.Version, config.GithubLatestRelease))
		if err != nil {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	latestVersion, err := GetLatest(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if latestVersion != nil && *latestVersion != config.Version {
		// prints to stderr to not pollute output redirection on scaffold
		println(color.YellowString("📣📦 a newer version of krknctl, %s, is currently available, check it out! %s", *latestVersion, config.GithubLatestRelease))
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
