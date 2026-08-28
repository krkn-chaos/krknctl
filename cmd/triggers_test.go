package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/triggers"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

// TestDummyScenarioHasNoTriggerFlags verifies Phase 3 backward compatibility:
// fixture metadata does not declare trigger support, so trigger flags must
// not be considered discoverable for it.
func TestDummyScenarioHasNoTriggerFlags(t *testing.T) {
	loadFields := func(t *testing.T, name string) []typing.InputField {
		t.Helper()
		path := filepath.Join("..", "tests", "containerfiles", "dummyscenario", name)
		data, err := os.ReadFile(path)
		assert.Nil(t, err)
		var fields []typing.InputField
		assert.Nil(t, json.Unmarshal(data, &fields))
		return fields
	}

	scenarioFields := loadFields(t, "krknctl-input.json")
	globalFields := loadFields(t, "krknctl-global-input.json")

	assert.False(t, triggers.SupportsTriggers(scenarioFields))
	assert.False(t, triggers.SupportsTriggers(globalFields))

	for _, field := range append(scenarioFields, globalFields...) {
		if field.Name == nil {
			continue
		}
		for _, triggerFlag := range triggers.KnownTriggerFlagNames() {
			assert.NotEqual(t, triggerFlag, *field.Name)
		}
	}
}

// TestParseFlagsMapsTriggerEnvVars verifies Phase 3 mapping: when trigger
// fields exist in scenario metadata, ParseFlags sets the matching env vars.
func TestParseFlagsMapsTriggerEnvVars(t *testing.T) {
	fields := []typing.InputField{
		{
			Name:             strPtr(triggers.FlagTriggerPromQuery),
			ShortDescription: strPtr("Prometheus Trigger Query"),
			Description:      strPtr("PromQL expression"),
			Variable:         strPtr(triggers.EnvTriggerPromQuery),
			Type:             typing.String,
			Default:          strPtr(""),
		},
		{
			Name:             strPtr(triggers.FlagTriggersTimeout),
			ShortDescription: strPtr("Trigger Timeout"),
			Description:      strPtr("Max seconds to wait"),
			Variable:         strPtr(triggers.EnvTriggersTimeout),
			Type:             typing.Number,
			Default:          strPtr("0"),
		},
		{
			Name:             strPtr(triggers.FlagTriggersInterval),
			ShortDescription: strPtr("Trigger Poll Interval"),
			Description:      strPtr("Seconds between checks"),
			Variable:         strPtr(triggers.EnvTriggersInterval),
			Type:             typing.Number,
			Default:          strPtr("5"),
		},
		{
			Name:             strPtr(triggers.FlagTriggersMode),
			ShortDescription: strPtr("Trigger Mode"),
			Description:      strPtr("all_of or any_of"),
			Variable:         strPtr(triggers.EnvTriggersMode),
			Type:             typing.Enum,
			AllowedValues:    strPtr("all_of,any_of"),
			Separator:        strPtr(","),
			Default:          strPtr("all_of"),
		},
		{
			Name:             strPtr(triggers.FlagTriggersOnTimeout),
			ShortDescription: strPtr("Timeout Behavior"),
			Description:      strPtr("skip, fail, or run_anyway"),
			Variable:         strPtr(triggers.EnvTriggersOnTimeout),
			Type:             typing.Enum,
			AllowedValues:    strPtr("skip,fail,run_anyway"),
			Separator:        strPtr(","),
			Default:          strPtr("skip"),
		},
		{
			Name:             strPtr(triggers.FlagPrometheusBearerToken),
			ShortDescription: strPtr("Prometheus Bearer Token"),
			Description:      strPtr("Bearer token"),
			Variable:         strPtr(triggers.EnvPrometheusBearerToken),
			Type:             typing.String,
			Default:          strPtr(""),
			Secret:           true,
		},
	}

	assert.True(t, triggers.SupportsTriggers(fields))

	scenario := &models.ScenarioDetail{Fields: fields}
	collected := make(map[string]*string)
	flagSet := pflag.NewFlagSet("scenario", pflag.ContinueOnError)
	for _, field := range fields {
		defaultValue := ""
		if field.Default != nil {
			defaultValue = *field.Default
		}
		collected[*field.Name] = flagSet.String(*field.Name, defaultValue, *field.Description)
	}

	query := `avg(rate(container_cpu_usage_seconds_total[5m])) > 0.8`
	args := []string{
		"node-cpu-hog",
		"--" + triggers.FlagTriggerPromQuery, query,
		"--" + triggers.FlagTriggersTimeout, "600",
		"--" + triggers.FlagTriggersInterval, "10",
		"--" + triggers.FlagTriggersMode, "all_of",
		"--" + triggers.FlagTriggersOnTimeout, "run_anyway",
		"--" + triggers.FlagPrometheusBearerToken, "tok",
	}

	environment, _, err := ParseFlags(scenario, args, collected, true)
	assert.Nil(t, err)
	assert.NotNil(t, environment)

	env := *environment
	assert.Equal(t, query, env[triggers.EnvTriggerPromQuery].value)
	assert.Equal(t, "600", env[triggers.EnvTriggersTimeout].value)
	assert.Equal(t, "10", env[triggers.EnvTriggersInterval].value)
	assert.Equal(t, "all_of", env[triggers.EnvTriggersMode].value)
	assert.Equal(t, "run_anyway", env[triggers.EnvTriggersOnTimeout].value)
	assert.Equal(t, "tok", env[triggers.EnvPrometheusBearerToken].value)
	assert.True(t, env[triggers.EnvPrometheusBearerToken].secret)

	// Without trigger args and skipDefault=true, no trigger env vars are set.
	environment, _, err = ParseFlags(scenario, []string{"node-cpu-hog"}, collected, true)
	assert.Nil(t, err)
	_, hasQuery := (*environment)[triggers.EnvTriggerPromQuery]
	assert.False(t, hasQuery)
}
