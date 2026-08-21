package cmd

import (
	"strings"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
)

func TestParseWeightFlag_Valid(t *testing.T) {
	tests := []struct {
		name           string
		flag           string
		expectedName   string
		expectedWeight float64
	}{
		{"integer weight", "scenario-1=2", "scenario-1", 2.0},
		{"decimal weight", "scenario-2=2.5", "scenario-2", 2.5},
		{"leading decimal", "scenario-3=.5", "scenario-3", 0.5},
		{"with hyphens", "pod-scenarios=3.0", "pod-scenarios", 3.0},
		{"with underscores", "node_scenarios=1.5", "node_scenarios", 1.5},
		{"mixed naming", "test-scenario_1=4.2", "test-scenario_1", 4.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, weight, err := parseWeightFlag(tt.flag)
			if err != nil {
				t.Errorf("parseWeightFlag() unexpected error: %v", err)
				return
			}
			if name != tt.expectedName {
				t.Errorf("parseWeightFlag() name = %v, want %v", name, tt.expectedName)
			}
			if weight != tt.expectedWeight {
				t.Errorf("parseWeightFlag() weight = %v, want %v", weight, tt.expectedWeight)
			}
		})
	}
}

func TestParseWeightFlag_InvalidFormat(t *testing.T) {
	tests := []struct {
		name        string
		flag        string
		expectedErr string
	}{
		{"missing equals", "scenario1", "invalid --weight format"},
		{"missing weight", "scenario-1=", "invalid --weight format"},
		{"missing scenario", "=2.0", "invalid --weight format"},
		{"wrong separator", "scenario-1:2.0", "invalid --weight format"},
		{"special chars in name", "scenario@1=2.0", "invalid --weight format"},
		{"spaces in name", "my scenario=2.0", "invalid --weight format"},
		{"invalid weight chars", "scenario-1=2.0a", "invalid --weight format"},
		{"multiple equals", "scenario-1=2=3", "invalid --weight format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseWeightFlag(tt.flag)
			if err == nil {
				t.Errorf("parseWeightFlag() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("parseWeightFlag() error = %v, want substring %v", err, tt.expectedErr)
			}
		})
	}
}

func TestParseWeightFlag_InvalidWeight(t *testing.T) {
	tests := []struct {
		name        string
		flag        string
		expectedErr string
	}{
		{"zero weight", "scenario-1=0", "weight must be greater than 0"},
		{"negative weight", "scenario-1=-1.5", "invalid --weight format"}, // Regex won't match negative
		{"very small weight", "scenario-1=0.0", "weight must be greater than 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseWeightFlag(tt.flag)
			if err == nil {
				t.Errorf("parseWeightFlag() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("parseWeightFlag() error = %v, want substring %v", err, tt.expectedErr)
			}
		})
	}
}

func TestParseAndApplyWeightOverrides_Success(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-1", ResiliencyWeight: 1.0},
		},
		"scenario-2": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-2", ResiliencyWeight: 1.0},
		},
		"scenario-3": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-3"},
		},
	}

	weightFlags := []string{"scenario-1=2.0", "scenario-2=3.5"}

	err := ParseAndApplyWeightOverrides(scenarios, weightFlags)
	if err != nil {
		t.Errorf("ParseAndApplyWeightOverrides() unexpected error: %v", err)
		return
	}

	if scenarios["scenario-1"].ResiliencyWeight != 2.0 {
		t.Errorf("scenario-1 weight = %v, want 2.0", scenarios["scenario-1"].ResiliencyWeight)
	}
	if scenarios["scenario-2"].ResiliencyWeight != 3.5 {
		t.Errorf("scenario-2 weight = %v, want 3.5", scenarios["scenario-2"].ResiliencyWeight)
	}
	if scenarios["scenario-3"].ResiliencyWeight != 0.0 {
		t.Errorf("scenario-3 weight = %v, want 0.0 (unchanged)", scenarios["scenario-3"].ResiliencyWeight)
	}
}

func TestParseAndApplyWeightOverrides_UnknownScenario(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-1"},
		},
		"scenario-2": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-2"},
		},
	}

	weightFlags := []string{"scenario-1=2.0", "unknown-scenario=3.0"}

	err := ParseAndApplyWeightOverrides(scenarios, weightFlags)
	if err == nil {
		t.Errorf("ParseAndApplyWeightOverrides() expected error for unknown scenario, got nil")
		return
	}

	if !strings.Contains(err.Error(), "unknown scenario") {
		t.Errorf("ParseAndApplyWeightOverrides() error = %v, want substring 'unknown scenario'", err)
	}
	if !strings.Contains(err.Error(), "unknown-scenario") {
		t.Errorf("ParseAndApplyWeightOverrides() error should mention 'unknown-scenario'")
	}
	if !strings.Contains(err.Error(), "Available scenarios:") {
		t.Errorf("ParseAndApplyWeightOverrides() error should list available scenarios")
	}
}

func TestParseAndApplyWeightOverrides_DuplicateScenario(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-1", ResiliencyWeight: 1.0},
		},
	}

	// Last value should win
	weightFlags := []string{"scenario-1=2.0", "scenario-1=3.0"}

	err := ParseAndApplyWeightOverrides(scenarios, weightFlags)
	if err != nil {
		t.Errorf("ParseAndApplyWeightOverrides() unexpected error: %v", err)
		return
	}

	if scenarios["scenario-1"].ResiliencyWeight != 3.0 {
		t.Errorf("scenario-1 weight = %v, want 3.0 (last value should win)", scenarios["scenario-1"].ResiliencyWeight)
	}
}

func TestParseAndApplyWeightOverrides_EmptyFlags(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-1", ResiliencyWeight: 1.5},
		},
	}

	originalWeight := scenarios["scenario-1"].ResiliencyWeight

	err := ParseAndApplyWeightOverrides(scenarios, []string{})
	if err != nil {
		t.Errorf("ParseAndApplyWeightOverrides() unexpected error: %v", err)
		return
	}

	if scenarios["scenario-1"].ResiliencyWeight != originalWeight {
		t.Errorf("scenario-1 weight = %v, want %v (unchanged)", scenarios["scenario-1"].ResiliencyWeight, originalWeight)
	}
}

func TestParseAndApplyWeightOverrides_InvalidFlag(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{
			Scenario: models.Scenario{Name: "scenario-1"},
		},
	}

	tests := []struct {
		name        string
		flags       []string
		expectedErr string
	}{
		{"invalid format", []string{"invalid-format"}, "invalid --weight format"},
		{"zero weight", []string{"scenario-1=0"}, "weight must be greater than 0"},
		{"invalid weight", []string{"scenario-1=abc"}, "invalid --weight format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseAndApplyWeightOverrides(scenarios, tt.flags)
			if err == nil {
				t.Errorf("ParseAndApplyWeightOverrides() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("ParseAndApplyWeightOverrides() error = %v, want substring %v", err, tt.expectedErr)
			}
		})
	}
}

func TestGetScenarioNames(t *testing.T) {
	scenarios := models.ScenarioSet{
		"scenario-1": models.ScenarioNode{Scenario: models.Scenario{Name: "scenario-1"}},
		"scenario-2": models.ScenarioNode{Scenario: models.Scenario{Name: "scenario-2"}},
		"scenario-3": models.ScenarioNode{Scenario: models.Scenario{Name: "scenario-3"}},
	}

	names := getScenarioNames(scenarios)

	// Check that all scenario names are included
	for name := range scenarios {
		if !strings.Contains(names, name) {
			t.Errorf("getScenarioNames() = %v, missing scenario %v", names, name)
		}
	}

	// Check comma separation (should have 2 commas for 3 items)
	commaCount := strings.Count(names, ",")
	if commaCount != 2 {
		t.Errorf("getScenarioNames() comma count = %v, want 2", commaCount)
	}
}

func TestGetScenarioNames_Empty(t *testing.T) {
	scenarios := models.ScenarioSet{}
	names := getScenarioNames(scenarios)

	if names != "" {
		t.Errorf("getScenarioNames() = %v, want empty string", names)
	}
}
