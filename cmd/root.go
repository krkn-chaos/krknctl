package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	"os"
)

func Execute(providerFactory *factory.ProviderFactory, scenarioOrchestrator *scenario_orchestrator.ScenarioOrchestrator, config config.Config) {

	rootCmd := NewRootCommand(providerFactory, config)
	// TODO: json output + offline repos
	/*
		var jsonFlag bool
		var offlineFlag bool
		var offlineRepoConfig string
		rootCmd.PersistentFlags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON")
		rootCmd.PersistentFlags().BoolVarP(&offlineFlag, "offline", "o", false, "Offline mode")
		rootCmd.PersistentFlags().StringVarP(&offlineRepoConfig, "offline-repo-config", "r", "", "Offline repository config file")
		rootCmd.MarkFlagsRequiredTogether("offline", "offline-repo-config")
	*/

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
	graphCmd := NewGraphCommand(providerFactory, config)
	graphRunCmd := NewGraphRunCommand(providerFactory, scenarioOrchestrator, config)
	graphRunCmd.Flags().String("kubeconfig", "", "kubeconfig path (if not set will default to ~/.kube/config)")
	graphRunCmd.Flags().String("alerts-profile", "", "custom alerts profile file path")
	graphRunCmd.Flags().String("metrics-profile", "", "custom metrics profile file path")

	graphScaffoldCmd := NewGraphScaffoldCommand(providerFactory, config)
	graphCmd.AddCommand(graphRunCmd)
	graphCmd.AddCommand(graphScaffoldCmd)
	rootCmd.AddCommand(graphCmd)

	attachCmd := NewAttachCmd(scenarioOrchestrator, config)
	rootCmd.AddCommand(attachCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
