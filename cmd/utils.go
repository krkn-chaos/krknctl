package cmd

import (
	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
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

func GetProvider(offline bool, providerFactory *factory.ProviderFactory) factory.ScenarioDataProvider {
	var provider factory.ScenarioDataProvider
	if offline {
		provider = providerFactory.NewInstance(factory.Offline)
	} else {
		provider = providerFactory.NewInstance(factory.Online)
	}
	return provider
}

func FetchScenarios(provider factory.ScenarioDataProvider, dataSource string) (*[]string, error) {
	scenarios, err := provider.GetScenarios(dataSource)
	if err != nil {
		return nil, err
	}
	var foundScenarios []string
	for _, scenario := range *scenarios {
		foundScenarios = append(foundScenarios, scenario.Name)
	}
	return &foundScenarios, nil
}

func BuildDataSource(config config.Config, offline bool, offlineSource *string) string {
	var dataSource = ""
	if offline == true {
		if offlineSource != nil {
			dataSource = *offlineSource
		} else {
			dataSource = ""
		}
	} else {
		dataSource = config.QuayApi + "/" + config.QuayRegistry
	}
	return dataSource
}
