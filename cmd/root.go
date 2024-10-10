package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var jsonFlag bool
var offlineFlag bool
var offlineRepoConfig string
var rootCmd = &cobra.Command{
	Use:   "krknctl",
	Short: "krkn CLI",
	Long:  `krkn Command Line Interface`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	rootCmd.PersistentFlags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON")
	rootCmd.PersistentFlags().BoolVarP(&offlineFlag, "offline", "o", false, "Offline mode")
	rootCmd.PersistentFlags().StringVarP(&offlineRepoConfig, "offline-repo-config", "r", "", "Offline repository config file")
	rootCmd.MarkFlagsRequiredTogether("offline", "offline-repo-config")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
