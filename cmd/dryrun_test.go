package cmd

import (
	"strings"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/stretchr/testify/assert"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }

// makeField builds a typing.InputField directly without JSON round-tripping,
// so Required and Default are never silently reset to zero values.
func makeField(name, variable string, typ typing.Type, required bool, defaultVal *string) typing.InputField {
	return typing.InputField{
		Name:        strPtr(name),
		Variable:    strPtr(variable),
		Description: strPtr(name + " description"),
		Type:        typ,
		Required:    required,
		Default:     defaultVal,
	}
}

func emptyGlobal() *models.ScenarioDetail {
	return &models.ScenarioDetail{}
}

// ── parseDryRunFlag ───────────────────────────────────────────────────────────

func TestParseDryRunFlag_NotPresent(t *testing.T) {
	_, found, err := parseDryRunFlag([]string{"my-scenario", "--kubeconfig", "/tmp/kube"})
	assert.Nil(t, err)
	assert.False(t, found)
}

func TestParseDryRunFlag_ValidClient(t *testing.T) {
	_, found, err := parseDryRunFlag([]string{"my-scenario", "--dry-run=client"})
	assert.Nil(t, err)
	assert.True(t, found)
}

func TestParseDryRunFlag_ValidClientSpaceSeparated(t *testing.T) {
	_, found, err := parseDryRunFlag([]string{"my-scenario", "--dry-run", "client"})
	assert.Nil(t, err)
	assert.True(t, found)
}

func TestParseDryRunFlag_InvalidValue(t *testing.T) {
	_, found, err := parseDryRunFlag([]string{"my-scenario", "--dry-run=server"})
	assert.NotNil(t, err)
	assert.False(t, found)
	assert.Contains(t, err.Error(), "client")
}

func TestParseDryRunFlag_MissingValue(t *testing.T) {
	_, found, err := parseDryRunFlag([]string{"my-scenario", "--dry-run"})
	assert.NotNil(t, err)
	assert.False(t, found)
}

// ── validateScenarioLocally ───────────────────────────────────────────────────

func TestValidateScenarioLocally_NilScenario(t *testing.T) {
	result := validateScenarioLocally(nil, emptyGlobal(), []string{})
	assert.False(t, result.Valid)
	assertHasFail(t, result, "schema not found")
}

func TestValidateScenarioLocally_ValidConfig(t *testing.T) {
	dur := "10"
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("duration", "DURATION", typing.Number, true, &dur),
		},
	}
	args := []string{"my-scenario", "--duration=30"}
	result := validateScenarioLocally(scenario, emptyGlobal(), args)
	assert.True(t, result.Valid)
	assertHasOK(t, result, "Scenario schema valid")
	assertHasOK(t, result, "All required fields present")
	assertHasOK(t, result, "Values validated")
}

func TestValidateScenarioLocally_MissingRequiredField(t *testing.T) {
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("namespace", "NAMESPACE", typing.String, true, nil),
		},
	}
	result := validateScenarioLocally(scenario, emptyGlobal(), []string{"my-scenario"})
	assert.False(t, result.Valid)
	assertHasFail(t, result, "namespace")
}

func TestValidateScenarioLocally_RequiredFieldWithDefault_Valid(t *testing.T) {
	def := "default-ns"
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("namespace", "NAMESPACE", typing.String, true, &def),
		},
	}
	// no --namespace supplied; default covers the required constraint
	result := validateScenarioLocally(scenario, emptyGlobal(), []string{"my-scenario"})
	assert.True(t, result.Valid)
}

func TestValidateScenarioLocally_InvalidType_Number(t *testing.T) {
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("duration", "DURATION", typing.Number, false, nil),
		},
	}
	args := []string{"my-scenario", "--duration=notanumber"}
	result := validateScenarioLocally(scenario, emptyGlobal(), args)
	assert.False(t, result.Valid)
	assertHasFail(t, result, "duration")
}

func TestValidateScenarioLocally_InvalidType_Boolean(t *testing.T) {
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("verbose", "VERBOSE", typing.Boolean, false, nil),
		},
	}
	args := []string{"my-scenario", "--verbose=maybe"}
	result := validateScenarioLocally(scenario, emptyGlobal(), args)
	assert.False(t, result.Valid)
	assertHasFail(t, result, "verbose")
}

func TestValidateScenarioLocally_GlobalFieldsValidated(t *testing.T) {
	scenario := &models.ScenarioDetail{}
	global := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("log-level", "LOG_LEVEL", typing.Number, false, nil),
		},
	}
	args := []string{"my-scenario", "--log-level=bad"}
	result := validateScenarioLocally(scenario, global, args)
	assert.False(t, result.Valid)
	assertHasFail(t, result, "log-level")
}

func TestValidateScenarioLocally_NilGlobalDetail(t *testing.T) {
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("duration", "DURATION", typing.Number, false, nil),
		},
	}
	// nil globalDetail must not panic
	result := validateScenarioLocally(scenario, nil, []string{"my-scenario", "--duration=5"})
	assert.True(t, result.Valid)
}

func TestValidateScenarioLocally_MultipleErrors(t *testing.T) {
	scenario := &models.ScenarioDetail{
		Fields: []typing.InputField{
			makeField("namespace", "NAMESPACE", typing.String, true, nil),
			makeField("duration", "DURATION", typing.Number, true, nil),
		},
	}
	// both required, neither supplied
	result := validateScenarioLocally(scenario, emptyGlobal(), []string{"my-scenario"})
	assert.False(t, result.Valid)
	failCount := 0
	for _, m := range result.Messages {
		if !m.ok {
			failCount++
		}
	}
	assert.Equal(t, 2, failCount)
}

// ── DryRunResult.Print (smoke test — just ensure no panic) ───────────────────

func TestDryRunResult_Print_NoFields(t *testing.T) {
	r := &DryRunResult{Valid: true}
	assert.NotPanics(t, func() { r.Print() })
}

func TestDryRunResult_Print_Mixed(t *testing.T) {
	r := &DryRunResult{Valid: false}
	r.addOK("Scenario schema valid")
	r.addFail("Missing required field: namespace")
	assert.NotPanics(t, func() { r.Print() })
}

// ── assertion helpers ─────────────────────────────────────────────────────────

func assertHasOK(t *testing.T, r *DryRunResult, substr string) {
	t.Helper()
	for _, m := range r.Messages {
		if m.ok && containsCI(m.text, substr) {
			return
		}
	}
	t.Errorf("expected an OK message containing %q, got: %+v", substr, r.Messages)
}

func assertHasFail(t *testing.T, r *DryRunResult, substr string) {
	t.Helper()
	for _, m := range r.Messages {
		if !m.ok && containsCI(m.text, substr) {
			return
		}
	}
	t.Errorf("expected a FAIL message containing %q, got: %+v", substr, r.Messages)
}

func containsCI(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
