package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"os"
)

func Execute(factory *factory.ProviderFactory, config config.Config) {
	var jsonFlag bool
	var offlineFlag bool
	var offlineRepoConfig string
	rootCmd := NewRootCommand(factory)
	rootCmd.PersistentFlags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON")
	rootCmd.PersistentFlags().BoolVarP(&offlineFlag, "offline", "o", false, "Offline mode")
	rootCmd.PersistentFlags().StringVarP(&offlineRepoConfig, "offline-repo-config", "r", "", "Offline repository config file")
	rootCmd.MarkFlagsRequiredTogether("offline", "offline-repo-config")

	listCmd := NewListCommand(factory, config)
	rootCmd.AddCommand(listCmd)

	describeCmd := NewDescribeCommand(factory, config)
	rootCmd.AddCommand(describeCmd)

	runCmd := NewRunCommand(factory, config)
	runCmd.DisableFlagParsing = true
	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
