package cmd

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
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

func NewEnvironmentTable(env map[string]ParsedField, config config.Config) table.Table {
	tbl := table.New("Environment Value", "Value")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for k, v := range env {
		value := reduceString(v.value, config)
		if v.secret {
			tbl.AddRow(k, utils.MaskString(value))
		} else {
			tbl.AddRow(k, value)
		}

	}
	return tbl

}

func reduceString(value string, config config.Config) string {
	if len(value) >= config.TableFieldMaxLength {
		reducedValue := value[0:config.TableFieldMaxLength]
		return fmt.Sprintf("%s...(%d bytes more)", reducedValue, len(value)-config.TableFieldMaxLength)
	}
	return value
}

func NewGraphTable(graph [][]string, config config.Config) (table.Table, error) {
	tbl := table.New("Step", "Scenario ID")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)
	for i, v := range graph {
		if len(v) >= config.TableMaxStepScenarioLength {
			var err error
			v, err = groupScenarioLayer(v)
			if err != nil {
				return nil, err
			}
		}
		tbl.AddRow(i, strings.Join(v, ", "))
	}
	return tbl, nil
}

func groupScenarioLayer(layer []string) ([]string, error) {

	layerIndex := make(map[string]int)
	compressedLayers := make([]string, 0)
	scenarioIndex := make(map[string]bool)

	for _, srcGraphScenario := range layer {
		for _, dstGraphScenario := range layer {
			if srcGraphScenario != dstGraphScenario {
				cmnPrefix := commonPrefix(srcGraphScenario, dstGraphScenario)
				if len(cmnPrefix) > 0 {
					if ok := scenarioIndex[srcGraphScenario]; !ok {
						if _, ok = layerIndex[cmnPrefix]; ok {
							layerIndex[cmnPrefix]++
						} else {
							layerIndex[cmnPrefix] = 1
						}
						scenarioIndex[srcGraphScenario] = true
					}

				}
			}
		}
	}

	for k, v := range layerIndex {
		compressedLayers = append(compressedLayers, fmt.Sprintf("(%d) %s", v, k))
	}
	return compressedLayers, nil
}

func commonPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			prefix := a[:i]
			prefix = strings.TrimSuffix(prefix, "-")
			return prefix
		}
	}
	prefix := a[:minLen]
	prefix = strings.TrimSuffix(prefix, "-")
	return prefix
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
