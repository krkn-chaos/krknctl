package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestratorModels "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewSpinnerWithSuffix(suffix string) *spinner.Spinner {
	var s *spinner.Spinner = nil
	s = spinner.New(spinner.CharSets[39], 100*time.Millisecond)
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

func GetProvider(private bool, providerFactory *factory.ProviderFactory) provider.ScenarioDataProvider {
	var dataProvider provider.ScenarioDataProvider
	if private {
		dataProvider = providerFactory.NewInstance(provider.Private)
	} else {
		dataProvider = providerFactory.NewInstance(provider.Quay)
	}
	return dataProvider
}

func FetchScenarios(provider provider.ScenarioDataProvider, registrySettings *models.RegistryV2) (*[]string, error) {
	scenarios, err := provider.GetRegistryImages(registrySettings)
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
	fmt.Print("parse flags")
	for k := range scenarioCollectedFlags {
		field := scenarioDetail.GetFieldByName(k)
		fmt.Printf("get field %v", field)
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
			fmt.Printf("field validate %v value", value)
			if err != nil {
				fmt.Printf("err in %v validation", err)
				return nil, nil, err
			}
		}

		if value != nil && field.Type != typing.File {
			environment[*field.Variable] = *value
		} else if value != nil && field.Type == typing.File {
			if field.MountPath != nil {
				fmt.Printf("field value: %v", *value)
				volumes[*field.MountPath] = *value
				environment[*field.Variable] = *value
			} else {
				fileSrcDst := strings.Split(*value, ":")
				volumes[fileSrcDst[0]] = fileSrcDst[1]
			}
		}
	}

	return &environment, &volumes, nil
}

func parsePrivateRepoArgs(cmd *cobra.Command, args *[]string) (*models.RegistryV2, error) {

	var registrySettings *models.RegistryV2 = nil
	if cmd.DisableFlagParsing == false {
		var f *pflag.Flag = nil
		var privateRegistryFlag *pflag.Flag = nil
		privateRegistryFlag = cmd.Flags().Lookup("private-registry")
		if privateRegistryFlag != nil && privateRegistryFlag.Changed {
			registrySettings = &models.RegistryV2{}
			registrySettings.SkipTls = false
			registrySettings.RegistryUrl = privateRegistryFlag.Value.String()

		}

		f = cmd.Flags().Lookup("private-registry-username")
		if registrySettings != nil && f != nil {
			s := f.Value.String()
			if s != "" {
				registrySettings.Username = &s
			}
		}

		f = cmd.Flags().Lookup("private-registry-password")
		if registrySettings != nil && f != nil {
			s := f.Value.String()
			if s != "" {
				registrySettings.Password = &s
			}
		}

		f = cmd.Flags().Lookup("private-registry-skip-tls")
		if registrySettings != nil && f != nil && f.Changed {
			registrySettings.SkipTls = true
		}

		f = cmd.Flags().Lookup("private-registry-insecure")
		if registrySettings != nil && f != nil && f.Changed {
			registrySettings.Insecure = true
		}

		f = cmd.Flags().Lookup("private-registry-token")
		if registrySettings != nil && f != nil {
			s := f.Value.String()
			if s != "" {
				registrySettings.Token = &s
			}
		}

		f = cmd.Flags().Lookup("private-registry-scenarios")
		if registrySettings != nil {
			registrySettings.ScenarioRepository = f.Value.String()
			if registrySettings.ScenarioRepository == "" {
				return nil, errors.New("`private-registry-scenarios` must be set in private registry mode")
			}
		}
	} else {
		if args != nil {
			for i, a := range *args {
				if strings.HasPrefix(a, "--") {
					if a == "--private-registry" {
						registrySettings = &models.RegistryV2{}
						registrySettings.SkipTls = false
						if err := checkStringArgValue(*args, i); err != nil {
							return nil, err
						}
						registrySettings.RegistryUrl = (*args)[i+1]
					}
					if registrySettings != nil && a == "--private-registry-username" {
						if err := checkStringArgValue(*args, i); err != nil {
							return nil, err
						}
						v := (*args)[i+1]
						registrySettings.Username = &v
					}
					if registrySettings != nil && a == "--private-registry-password" {
						if err := checkStringArgValue(*args, i); err != nil {
							return nil, err
						}
						v := (*args)[i+1]
						registrySettings.Password = &v
					}
					if registrySettings != nil && a == "--private-registry-skip-tls" {
						registrySettings.SkipTls = true
					}

					if registrySettings != nil && a == "--private-registry-insecure" {
						registrySettings.Insecure = true
					}

					if registrySettings != nil && a == "--private-registry-token" {
						if err := checkStringArgValue(*args, i); err != nil {
							return nil, err
						}
						v := (*args)[i+1]
						registrySettings.Token = &v
					}

					if registrySettings != nil && a == "--private-registry-scenarios" {
						registrySettings.SkipTls = false
						if err := checkStringArgValue(*args, i); err != nil {
							return nil, err
						}
						registrySettings.ScenarioRepository = (*args)[i+1]
					}
				}
			}
		}
		if registrySettings != nil && registrySettings.ScenarioRepository == "" {
			return nil, errors.New("`private-registry-scenarios` must be set in private registry mode")
		}

	}
	return registrySettings, nil

}

func logPrivateRegistry(registry string) {
	log := fmt.Sprintf("[🔐 Private Registry] %s\n", registry)
	fmt.Println(log)
}

func validateGraphScenarioInput(provider provider.ScenarioDataProvider,
	nodes map[string]orchestratorModels.ScenarioNode,
	scenarioNameChannel chan *struct {
		name *string
		err  error
	},
	registrySettings *providermodels.RegistryV2) {
	for _, n := range nodes {
		// skip _comment
		if n.Name == "" {
			continue
		}
		scenarioNameChannel <- &struct {
			name *string
			err  error
		}{name: &n.Name, err: nil}
		scenarioDetail, err := provider.GetScenarioDetail(n.Name, registrySettings)

		if err != nil {
			scenarioNameChannel <- &struct {
				name *string
				err  error
			}{name: &n.Name, err: err}
			return
		}

		if scenarioDetail == nil {
			scenarioNameChannel <- &struct {
				name *string
				err  error
			}{name: &n.Name, err: fmt.Errorf("scenario %s not found", n.Name)}
			return
		}

		globalDetail, err := provider.GetGlobalEnvironment(registrySettings, scenarioDetail.Name)
		if err != nil {
			scenarioNameChannel <- &struct {
				name *string
				err  error
			}{name: &n.Name, err: err}
			return
		}

		// adding the global env fields to the scenario fields so, if global env is
		// added to the scenario the validation is available

		scenarioDetail.Fields = append(scenarioDetail.Fields, globalDetail.Fields...)

		for k, v := range n.Env {
			field := scenarioDetail.GetFieldByEnvVar(k)
			if field == nil {

				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: fmt.Errorf("environment variable %s not found", k)}
				return
			}
			_, err := field.Validate(&v)
			if err != nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: err}
				return
			}
		}

		for k, v := range n.Volumes {
			field := scenarioDetail.GetFileFieldByMountPath(v)
			if field == nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: fmt.Errorf("no file parameter found of type field for scenario %s with mountPath %s", n.Name, v)}
				return
			}
			_, err := field.Validate(&k)
			if err != nil {
				scenarioNameChannel <- &struct {
					name *string
					err  error
				}{name: &n.Name, err: err}
				return
			}
		}
	}
	scenarioNameChannel <- nil
}
