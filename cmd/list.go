package cmd

import (
	provider_factory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"log"
)

func NewListCommand(factory *provider_factory.ProviderFactory) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List scenarios",
		Long:  `List available krkn-hub scenarios`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			offline, err := cmd.Flags().GetBool("offline")
			//offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
			if err != nil {
				return err
			}
			provider := GetProvider(offline, factory)
			s := NewSpinnerWithSuffix("fetching scenarios...")
			s.Start()
			scenarios, err := provider.GetScenarios()
			if err != nil {
				s.Stop()
				log.Fatalf("failed to fetch scenarios: %v", err)
			}
			s.Stop()
			test := NewScenarioTable(scenarios)
			(*test).Print()
			return nil
		},
	}
	return listCmd
}

func GetProvider(offline bool, factory *provider_factory.ProviderFactory) provider_factory.ScenarioDataProvider {
	var provider provider_factory.ScenarioDataProvider
	if offline {
		provider = factory.NewInstance(provider_factory.Offline)
	} else {
		provider = factory.NewInstance(provider_factory.Online)
	}
	return provider
}
