package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
)

// ParseAndApplyWeightOverrides parses weight flags and applies them to the scenario set
// Supports both = and : as separators, and both integer and float weight values
func ParseAndApplyWeightOverrides(scenarios map[string]models.ScenarioNode, weightFlags []string) error {
	if len(weightFlags) == 0 {
		return nil
	}

	// Build set of valid Scenario IDs (excluding _comment metadata)
	validIDs := make(map[string]bool)
	for id := range scenarios {
		if id != "_comment" {
			validIDs[id] = true
		}
	}

	// Parse all weight flags first
	weights := make(map[string]float64)
	for _, flag := range weightFlags {
		scenarioID, weight, err := parseWeightFlag(flag)
		if err != nil {
			return err
		}
		weights[scenarioID] = weight
	}

	// Validate scenario IDs exist and apply weights
	for scenarioID, weight := range weights {
		if !validIDs[scenarioID] {
			var availableIDs []string
			for id := range validIDs {
				availableIDs = append(availableIDs, id)
			}
			sort.Strings(availableIDs)
			return fmt.Errorf("unknown scenario %q in --weight flag. Available scenarios: %s",
				scenarioID, strings.Join(availableIDs, ", "))
		}

		// Update the scenario weight
		scenario := scenarios[scenarioID]
		scenario.ResiliencyWeight = weight
		scenarios[scenarioID] = scenario
	}

	return nil
}

// parseWeightFlag parses a single weight flag in format "scenario-id=weight" or "scenario-id:weight"
// Supports both integer and float weight values
func parseWeightFlag(flag string) (string, float64, error) {
	var parts []string

	if strings.Contains(flag, "=") {
		parts = strings.Split(flag, "=")
	} else if strings.Contains(flag, ":") {
		parts = strings.Split(flag, ":")
	} else {
		return "", 0, fmt.Errorf("invalid --weight format %q. Expected format: ScenarioID=weight or ScenarioID:weight (e.g., critical-service=2 or critical-service=2.5)", flag)
	}

	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid --weight format %q. Expected format: ScenarioID=weight or ScenarioID:weight (e.g., critical-service=2 or critical-service=2.5)", flag)
	}

	scenarioID := strings.TrimSpace(parts[0])
	weightStr := strings.TrimSpace(parts[1])

	// Validate that both parts are non-empty
	if scenarioID == "" || weightStr == "" {
		return "", 0, fmt.Errorf("invalid --weight format %q. Expected format: ScenarioID=weight or ScenarioID:weight (e.g., critical-service=2 or critical-service=2.5)", flag)
	}

	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid weight value for scenario %q: %w", scenarioID, err)
	}

	if weight <= 0 {
		return "", 0, fmt.Errorf("invalid weight value %.2f for scenario %q: weight must be greater than 0", weight, scenarioID)
	}

	return scenarioID, weight, nil
}

// ParseWeightOverridesByName parses weights by scenario name field (for scaffolding)
// Format: scenario-name=weight or scenario-name:weight
func ParseWeightOverridesByName(weightArgs []string) (map[string]float64, error) {
	overrides := make(map[string]float64)
	if len(weightArgs) == 0 {
		return overrides, nil
	}

	for _, arg := range weightArgs {
		var parts []string
		if strings.Contains(arg, "=") {
			parts = strings.Split(arg, "=")
		} else if strings.Contains(arg, ":") {
			parts = strings.Split(arg, ":")
		} else {
			return nil, fmt.Errorf("invalid --weight format %q. Expected format: scenario-name=weight or scenario-name:weight (e.g., pod-scenarios=2 or pod-scenarios=2.5)", arg)
		}

		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --weight format %q. Expected format: scenario-name=weight or scenario-name:weight (e.g., pod-scenarios=2 or pod-scenarios=2.5)", arg)
		}

		scenarioName := strings.TrimSpace(parts[0])
		weightStr := strings.TrimSpace(parts[1])

		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weight value for scenario %q: %w", scenarioName, err)
		}

		if weight <= 0 {
			return nil, fmt.Errorf("invalid weight value %.2f for scenario %q: weight must be greater than 0", weight, scenarioName)
		}

		overrides[scenarioName] = weight
	}

	return overrides, nil
}

// ApplyWeightOverridesByName applies weights to scaffolded scenarios by matching the "name" field
func ApplyWeightOverridesByName(scenarioSet map[string]models.ScenarioNode, overridesByName map[string]float64) {
	for scenarioID, node := range scenarioSet {
		if scenarioID == "_comment" {
			continue
		}
		if weight, found := overridesByName[node.Name]; found {
			node.ResiliencyWeight = weight
			scenarioSet[scenarioID] = node
		}
	}
}

// getScenarioNames returns comma-separated list of scenario IDs for error messages
// Excludes the _comment metadata entry
func getScenarioNames(scenarios map[string]models.ScenarioNode) string {
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		if name != "_comment" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
