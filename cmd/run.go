package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

func NewRunCommand(factory *factory.ProviderFactory) *cobra.Command {
	collectedFlags := make(map[string]*string)
	var runCmd = &cobra.Command{
		Use:                "run",
		Short:              "runs a scenario",
		Long:               `runs a scenario`,
		DisableFlagParsing: false,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			offline, err := cmd.Flags().GetBool("offline")
			//offlineRepo, err := cmd.Flags().GetString("offline-repo-config")
			if err != nil {
				return []string{}, cobra.ShellCompDirectiveError
			}
			provider := GetProvider(offline, factory)
			scenarios, err := FetchScenarios(provider)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}
			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},

		PreRunE: func(cmd *cobra.Command, args []string) error {
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}
			provider := GetProvider(offline, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0])
			if err != nil {
				return err
			}
			if scenarioDetail == nil {
				return fmt.Errorf("%s scenario not found", args[0])
			}

			for _, field := range scenarioDetail.Fields {
				var defaultValue string = ""
				if field.Default != nil {
					defaultValue = *field.Default
				}
				collectedFlags[*field.Name] = cmd.LocalFlags().String(*field.Name, defaultValue, *field.Description)
				if err != nil {
					return err
				}

			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			spinner := NewSpinnerWithSuffix("validating input...")
			offline, err := cmd.Flags().GetBool("offline")
			if err != nil {
				return err
			}
			spinner.Start()

			provider := GetProvider(offline, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0])
			if err != nil {
				return err
			}
			spinner.Stop()
			// default
			for k, _ := range collectedFlags {
				field := scenarioDetail.GetFieldByName(k)
				var foundArg *string = nil
				for i, a := range args {
					if a == fmt.Sprintf("--%s", k) {
						if len(args) < i+2 || strings.HasPrefix(args[i+1], "--") {
							return fmt.Errorf("%s has no value", args[i])
						}
						foundArg = &args[i+1]
					}
				}
				if field != nil {
					value, err := field.Validate(foundArg)
					if err != nil {
						return err
					}
					if value != nil {
						fmt.Println(fmt.Sprintf("%s: valid", *value))
					}

					if value == nil && field.Required == false {
						fmt.Println(fmt.Sprintf("%s: nil but not required", *field.Name))
					}
				}

			}
			return nil
		},
	}
	return runCmd
}
