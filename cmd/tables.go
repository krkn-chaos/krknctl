package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"strings"
)
import "github.com/rodaine/table"

var headerFmt = color.New(color.FgGreen, color.Underline).SprintfFunc()
var columnFmt = color.New(color.FgYellow).SprintfFunc()

func NewScenarioTable(scenarios *[]models.ScenarioTag) table.Table {
	tbl := table.New("Name", "Size", "Digest", "Last Modified")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, scenario := range *scenarios {
		tbl.AddRow(scenario.Name, scenario.Size, scenario.Digest, scenario.LastModified)
	}
	return tbl
}

func NewArgumentTable(inputFields []typing.InputField) table.Table {
	tbl := table.New("Name", "Type", "Description", "Required", "Default")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, inputField := range inputFields {
		default_value := ""
		if inputField.Default != nil {
			default_value = *inputField.Default
		}
		tbl.AddRow(fmt.Sprintf("--%s", *inputField.Name), inputField.Type.String(), *inputField.ShortDescription, inputField.Required, default_value)
	}
	return tbl
}

func NewEnvironmentTable(env map[string]string) table.Table {
	tbl := table.New("Environment Value", "Value")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for k, v := range env {
		tbl.AddRow(k, v)
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
