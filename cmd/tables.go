package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"strings"
	"time"
)
import "github.com/rodaine/table"

var headerFmt = color.New(color.FgGreen, color.Underline).SprintfFunc()
var columnFmt = color.New(color.FgYellow).SprintfFunc()

func NewScenarioTable(scenarios *[]models.ScenarioTag, private bool) table.Table {
	var tbl table.Table
	if private {
		tbl = table.New("Name")
	} else {
		tbl = table.New("Name", "Size", "Digest", "Last Modified")
	}

	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, scenario := range *scenarios {
		if private {
			tbl.AddRow(scenario.Name)
		} else {
			tbl.AddRow(scenario.Name, *scenario.Size, *scenario.Digest, *scenario.LastModified)
		}

	}
	return tbl
}

func NewArgumentTable(inputFields []typing.InputField) table.Table {
	tbl := table.New("Name", "Type", "Description", "Required", "Default")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, inputField := range inputFields {
		defaultValue := ""
		if inputField.Default != nil {
			defaultValue = *inputField.Default
		}
		tbl.AddRow(fmt.Sprintf("--%s", *inputField.Name), inputField.Type.String(), *inputField.ShortDescription, inputField.Required, defaultValue)
	}
	return tbl
}

func NewEnvironmentTable(env map[string]ParsedFlag) table.Table {
	tbl := table.New("Environment Value", "Value")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for k, v := range env {
		if v.secret {
			tbl.AddRow(k, MaskString(v.value))
		} else {
			tbl.AddRow(k, v.value)
		}
	}
	return tbl

}

func NewGraphTable(graph [][]string) table.Table {
	tbl := table.New("Step", "Scenario ID")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for i, v := range graph {
		tbl.AddRow(i, strings.Join(v, ", "))
	}
	return tbl
}

func NewRunningScenariosTable(runningScenarios []orchestratormodels.ScenarioContainer) table.Table {
	tbl := table.New("Scenario ID", "Scenario Name", "Running Since", "Container Name")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for i, v := range runningScenarios {
		t := time.Unix(v.Container.Started, 0)
		tbl.AddRow(i, v.Scenario.Name, time.Since(t), v.Container.Name)
	}
	return tbl
}
