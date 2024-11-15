package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/text"
	"github.com/spf13/cobra"
	"log"
)

func NewDescribeCommand(factory *factory.ProviderFactory, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "describe",
		Short: "describes a scenario",
		Long:  `describes a scenario`,
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			dataSource, err := BuildDataSource(config, false, nil)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			provider := GetProvider(false, factory)

			scenarios, err := FetchScenarios(provider, dataSource)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}

			return *scenarios, cobra.ShellCompDirectiveNoFileComp

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dataSource, err := BuildDataSource(config, false, nil)
			if err != nil {
				return err
			}
			spinner := NewSpinnerWithSuffix("fetching scenario details...")
			spinner.Start()
			provider := GetProvider(false, factory)
			scenarioDetail, err := provider.GetScenarioDetail(args[0], dataSource)
			if err != nil {
				return err
			}
			spinner.Stop()
			if scenarioDetail == nil {
				return fmt.Errorf("could not find %s scenario", args[0])
			}
			PrintScenarioDetail(scenarioDetail)
			return nil
		},
	}
	return command
}

func PrintScenarioDetail(scenarioDetail *models.ScenarioDetail) {
	fmt.Print("\n")
	_, _ = color.New(color.FgGreen, color.Underline).Println(scenarioDetail.Title)
	justifiedText := text.Justify(scenarioDetail.Description, 65)
	for _, line := range justifiedText {
		fmt.Println(line)
	}
	fmt.Print("\n")
	argumentTable := NewArgumentTable(scenarioDetail.Fields)
	argumentTable.Print()
	fmt.Print("\n")

}
