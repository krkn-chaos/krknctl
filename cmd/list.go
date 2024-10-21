package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	provider_factory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"log"
)

func NewListCommand(factory *provider_factory.ProviderFactory, config config.Config) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "list scenarios",
		Long:  `list available krkn-hub scenarios`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			offline, err := cmd.Flags().GetBool("offline")
			//offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
			if err != nil {
				return err
			}
			// WIP: datasource offline TBD
			dataSource := BuildDataSource(config, offline, nil)
			provider := GetProvider(offline, factory)
			s := NewSpinnerWithSuffix("fetching scenarios...")
			s.Start()
			scenarios, err := provider.GetScenarios(dataSource)
			if err != nil {
				s.Stop()
				log.Fatalf("failed to fetch scenarios: %v", err)
			}
			s.Stop()
			scenarioTable := NewScenarioTable(scenarios)
			scenarioTable.Print()
			fmt.Print("\n")
			return nil
		},
	}
	return listCmd
}
