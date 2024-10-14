package cmd

import (
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)
import "github.com/rodaine/table"

func NewScenarioTable(scenarios *[]models.ScenarioTag) *table.Table {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()
	tbl := table.New("Name", "Size", "Digest", "Last Modified")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for _, scenario := range *scenarios {
		tbl.AddRow(scenario.Name, scenario.Size, scenario.Digest, scenario.LastModified)
	}
	return &tbl
}
