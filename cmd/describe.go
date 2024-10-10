package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "describes a scenario",
	Long:  `Describes a scenario`,
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"mimmo", "memma"}, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			fmt.Println("{\"describe\":\"" + args[0] + "\"}")
		} else {
			fmt.Println("Listing scenarios " + args[0])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
