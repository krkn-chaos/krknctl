package cmd

import (
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"os"
	"time"
)

func NewSpinnerWithSuffix(suffix string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[39], 100*time.Millisecond)
	s.Suffix = suffix
	return s
}

func NewRootCommand(factory *factory.ProviderFactory) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "krknctl",
		Short: "krkn CLI",
		Long:  `krkn Command Line Interface`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return rootCmd
}

func Execute(factory *factory.ProviderFactory) {
	var jsonFlag bool
	var offlineFlag bool
	var offlineRepoConfig string
	rootCmd := NewRootCommand(factory)
	rootCmd.PersistentFlags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON")
	rootCmd.PersistentFlags().BoolVarP(&offlineFlag, "offline", "o", false, "Offline mode")
	rootCmd.PersistentFlags().StringVarP(&offlineRepoConfig, "offline-repo-config", "r", "", "Offline repository config file")
	rootCmd.MarkFlagsRequiredTogether("offline", "offline-repo-config")

	listCmd := NewListCommand(factory)
	rootCmd.AddCommand(listCmd)

	describeCmd := NewDescribeCommand(factory)
	rootCmd.AddCommand(describeCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
