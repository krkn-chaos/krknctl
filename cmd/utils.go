package cmd

import (
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/spf13/cobra"
	"os"
	"strings"
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

func FetchScenarios(provider provider.ScenarioDataProvider) (*[]string, error) {
	scenarios, err := provider.GetRegistryImages()
	if err != nil {
		return nil, err
	}
	var foundScenarios []string
	for _, scenario := range *scenarios {
		foundScenarios = append(foundScenarios, scenario.Name)
	}
	return &foundScenarios, nil
}

func CheckFileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func ParseFlags(scenarioDetail *models.ScenarioDetail, args []string, scenarioCollectedFlags map[string]*string, skipDefault bool) (vol *map[string]string, env *map[string]string, err error) {
	environment := make(map[string]string)
	volumes := make(map[string]string)
	for k := range scenarioCollectedFlags {
		field := scenarioDetail.GetFieldByName(k)
		if field == nil {
			return nil, nil, fmt.Errorf("field %s not found", k)
		}
		var foundArg *string = nil
		for i, a := range args {
			if a == fmt.Sprintf("--%s", k) {
				if len(args) < i+2 || strings.HasPrefix(args[i+1], "--") {
					return nil, nil, fmt.Errorf("%s has no value", args[i])
				}
				foundArg = &args[i+1]
			}
		}
		var value *string = nil
		if foundArg != nil || skipDefault == false {
			value, err = field.Validate(foundArg)
			if err != nil {
				return nil, nil, err
			}
		}

		if value != nil && field.Type != typing.File {
			environment[*field.Variable] = *value
		} else if value != nil && field.Type == typing.File {
			fileSrcDst := strings.Split(*value, ":")
			volumes[fileSrcDst[0]] = fileSrcDst[1]
		}

	}

	return &environment, &volumes, nil
}
