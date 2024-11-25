package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"github.com/spf13/cobra"
	"os"
)

func Execute(providerFactory *factory.ProviderFactory, scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) {

	rootCmd := NewRootCommand(config)
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

	graphScaffoldCmd := NewGraphScaffoldCommand(providerFactory, config)
	graphCmd.AddCommand(graphRunCmd)
	graphCmd.AddCommand(graphScaffoldCmd)
	rootCmd.AddCommand(graphCmd)

	attachCmd := NewAttachCmd(scenarioOrchestrator)
	rootCmd.AddCommand(attachCmd)
	queryCmd := NewQueryStatusCommand(scenarioOrchestrator, config)
	queryCmd.Flags().String("graph", "", "to query the exit status of a previously run graph file")
	rootCmd.AddCommand(queryCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
