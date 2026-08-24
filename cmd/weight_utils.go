package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
)

// weightFlagPattern matches "scenario-name=1.5" format
// Allows alphanumeric, hyphens, underscores in scenario names
var weightFlagPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+)=([0-9]*\.?[0-9]+)$`)

// ParseAndApplyWeightOverrides parses weight flags and updates scenario weights
// Returns error if any weight flag is invalid or references unknown scenario
func ParseAndApplyWeightOverrides(scenarios models.ScenarioSet, weightFlags []string) error {
	if len(weightFlags) == 0 {
		return nil
	}

	// Parse all weight flags first
	weights := make(map[string]float64)
	for _, flag := range weightFlags {
		scenarioName, weight, err := parseWeightFlag(flag)
		if err != nil {
			return err
		}
		weights[scenarioName] = weight
	}

	// Validate scenario names exist and apply weights
	for scenarioName, weight := range weights {
		scenario, exists := scenarios[scenarioName]
		if !exists {
			return fmt.Errorf("unknown scenario '%s' in --weight flag. Available scenarios: %s",
				scenarioName, getScenarioNames(scenarios))
		}

		// Update the scenario weight
		scenario.ResiliencyWeight = weight
		scenarios[scenarioName] = scenario
	}

	return nil
}

// parseWeightFlag parses a single weight flag in format "scenario-name=weight"
func parseWeightFlag(flag string) (string, float64, error) {
	matches := weightFlagPattern.FindStringSubmatch(flag)
	if matches == nil {
		return "", 0, fmt.Errorf("invalid --weight format '%s'. Expected format: scenario-name=weight (e.g., pod-scenarios=2.0)", flag)
	}

	scenarioName := matches[1]
	weightStr := matches[2]

	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid weight value '%s' in --weight flag: must be a positive number", weightStr)
	}

	if weight <= 0 {
		return "", 0, fmt.Errorf("invalid weight value %.2f for scenario '%s': weight must be greater than 0", weight, scenarioName)
	}

	return scenarioName, weight, nil
}

// getScenarioNames returns comma-separated list of scenario names for error messages
func getScenarioNames(scenarios models.ScenarioSet) string {
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
