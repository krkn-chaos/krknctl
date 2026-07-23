package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

func ValidateScenarioConfig(scenarioDetail *models.ScenarioDetail, globalDetail *models.ScenarioDetail, args []string) (bool, []string) {
	var validationErrors []string

	checkFields := func(detail *models.ScenarioDetail, skipDefault bool) {
		if detail == nil {
			return
		}
		for i := range detail.Fields {
			field := &detail.Fields[i]
			if field.Name == nil {
				continue
			}
			flagName := fmt.Sprintf("--%s", *field.Name)
			value, found, err := ParseArgValue(args, flagName)
			if err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("field `%s`: %s", *field.Name, err.Error()))
				continue
			}

			if !found && skipDefault {
				continue
			}

			var valuePtr *string
			if found {
				valuePtr = &value
			}

			if _, err := field.Validate(valuePtr); err != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("field `%s`: %s", *field.Name, err.Error()))
			}
		}
	}

	checkFields(scenarioDetail, false)
	checkFields(globalDetail, true)

	return len(validationErrors) == 0, validationErrors
}

func PrintDryRunResult(valid bool, validationErrors []string) error {
	if valid {
		for _, line := range []string{"✔ Scenario schema valid", "✔ All required fields present", "✔ Values validated"} {
			if _, err := color.New(color.FgGreen).Println(line); err != nil {
				return err
			}
		}
		return nil
	}

	for _, e := range validationErrors {
		if _, err := color.New(color.FgRed).Println(fmt.Sprintf("✖ %s", e)); err != nil {
			return err
		}
	}
	return nil
}