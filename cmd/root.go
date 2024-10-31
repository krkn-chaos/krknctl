package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"os"
)

func Execute(providerFactory *factory.ProviderFactory, containerManager *container_manager.ContainerManager, config config.Config) {

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

	listCmd := NewListCommand(providerFactory, config)
	rootCmd.AddCommand(listCmd)

	describeCmd := NewDescribeCommand(providerFactory, config)
	rootCmd.AddCommand(describeCmd)

	runCmd := NewRunCommand(providerFactory, containerManager, config)
	runCmd.LocalFlags().String("kubeconfig", "", "kubeconfig path (if not set will default to ~/.kube/config)")
	runCmd.LocalFlags().Bool("detached", false, "if set this flag will run in detached mode")
	runCmd.DisableFlagParsing = true
	rootCmd.AddCommand(runCmd)

	cleanCmd := NewCleanCommand(containerManager, config)
	rootCmd.AddCommand(cleanCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
