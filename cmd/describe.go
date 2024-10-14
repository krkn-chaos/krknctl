package cmd

import (
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
)

func NewDescribeCommand(factory *factory.ProviderFactory) *cobra.Command {
	var describeCmd = &cobra.Command{
		Use:   "describe",
		Short: "describes a scenario",
		Long:  `Describes a scenario`,
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			offline, err := cmd.Flags().GetBool("offline")
			//offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
			if err != nil {
				return []string{}, cobra.ShellCompDirectiveError
			}
			provider := GetProvider(offline, factory)
			scenarios, err := provider.GetScenarios()
			if err != nil {
				return []string{}, cobra.ShellCompDirectiveError
			}
			var foundScenarios []string
			for _, scenario := range *scenarios {
				foundScenarios = append(foundScenarios, scenario.Name)
			}
			return foundScenarios, cobra.ShellCompDirectiveNoFileComp

		},
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}
	return describeCmd
}
