package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
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
	tbl := table.New("Name", "Type", "Description", "Required")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, inputField := range inputFields {
		tbl.AddRow(fmt.Sprintf("--%s", *inputField.Name), inputField.Type.String(), *inputField.ShortDescription, inputField.Default == nil)
	}
	return tbl
}
