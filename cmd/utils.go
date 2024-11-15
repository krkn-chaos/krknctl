package cmd

import (
	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"time"
)

func NewSpinnerWithSuffix(suffix string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[39], 100*time.Millisecond)
	s.Suffix = suffix
	return s
}

func NewRootCommand(krknctlConfig config.Config) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:     "krknctl",
		Short:   "krkn CLI",
		Long:    `krkn Command Line Interface`,
		Version: krknctlConfig.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return rootCmd
}

func GetProvider(offline bool, providerFactory *factory.ProviderFactory) provider.ScenarioDataProvider {
	var dataProvider provider.ScenarioDataProvider
	if offline {
		dataProvider = providerFactory.NewInstance(provider.Offline)
	} else {
		dataProvider = providerFactory.NewInstance(provider.Online)
	}
	return dataProvider
}

func FetchScenarios(provider provider.ScenarioDataProvider, dataSource string) (*[]string, error) {
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
		dataSource = config.GetQuayRepositoryApiUri()
	}
	return dataSource
}
