package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/triggers"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

// TestDummyScenarioHasNoTriggerFlags verifies that scenarios without trigger
// metadata do not declare trigger support or expose trigger flags.
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
		if field.Group != nil {
			assert.NotEqual(t, triggers.GroupTriggers, *field.Group)
		}
		if field.Name != nil {
			assert.False(t, strings.HasPrefix(*field.Name, "trigger-"), "dummy scenario should not have trigger flag: %s", *field.Name)
			assert.False(t, strings.HasPrefix(*field.Name, "triggers-"), "dummy scenario should not have trigger flag: %s", *field.Name)
		}
	}
}

// TestParseFlagsMapsTriggerEnvVars verifies that when trigger fields exist
// in scenario metadata, ParseFlags sets the matching environment variables.
func TestParseFlagsMapsTriggerEnvVars(t *testing.T) {
	fields := []typing.InputField{
		{
			Name:             strPtr("trigger-command"),
			ShortDescription: strPtr("Trigger command"),
			Description:      strPtr("Shell command to evaluate before chaos starts"),
			Variable:         strPtr("TRIGGER_COMMAND"),
			Type:             typing.String,
			Default:          strPtr(""),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("trigger-prom-query"),
			ShortDescription: strPtr("Prometheus Trigger Query"),
			Description:      strPtr("PromQL expression"),
			Variable:         strPtr("TRIGGER_PROM_QUERY"),
			Type:             typing.String,
			Default:          strPtr(""),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("triggers-timeout"),
			ShortDescription: strPtr("Trigger Timeout"),
			Description:      strPtr("Max seconds to wait"),
			Variable:         strPtr("TRIGGERS_TIMEOUT"),
			Type:             typing.Number,
			Default:          strPtr("0"),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("triggers-interval"),
			ShortDescription: strPtr("Trigger Poll Interval"),
			Description:      strPtr("Seconds between checks"),
			Variable:         strPtr("TRIGGERS_INTERVAL"),
			Type:             typing.Number,
			Default:          strPtr("5"),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("triggers-mode"),
			ShortDescription: strPtr("Trigger Mode"),
			Description:      strPtr("all_of or any_of"),
			Variable:         strPtr("TRIGGERS_MODE"),
			Type:             typing.Enum,
			AllowedValues:    strPtr("all_of,any_of"),
			Separator:        strPtr(","),
			Default:          strPtr("all_of"),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("triggers-on-timeout"),
			ShortDescription: strPtr("Timeout Behavior"),
			Description:      strPtr("skip, fail, or run_anyway"),
			Variable:         strPtr("TRIGGERS_ON_TIMEOUT"),
			Type:             typing.Enum,
			AllowedValues:    strPtr("skip,fail,run_anyway"),
			Separator:        strPtr(","),
			Default:          strPtr("skip"),
			Group:            strPtr(triggers.GroupTriggers),
		},
		{
			Name:             strPtr("prometheus-bearer-token"),
			ShortDescription: strPtr("Prometheus Bearer Token"),
			Description:      strPtr("Bearer token"),
			Variable:         strPtr("PROMETHEUS_BEARER_TOKEN"),
			Type:             typing.String,
			Default:          strPtr(""),
			Secret:           true,
			Group:            strPtr(triggers.GroupTriggers),
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
	command := `curl -s http://app:8080/health | grep UP`
	args := []string{
		"node-cpu-hog",
		"--trigger-command", command,
		"--trigger-prom-query", query,
		"--triggers-timeout", "600",
		"--triggers-interval", "10",
		"--triggers-mode", "all_of",
		"--triggers-on-timeout", "run_anyway",
		"--prometheus-bearer-token", "tok",
	}

	environment, _, err := ParseFlags(scenario, args, collected, true)
	assert.Nil(t, err)
	assert.NotNil(t, environment)

	env := *environment
	assert.Equal(t, command, env["TRIGGER_COMMAND"].value)
	assert.Equal(t, query, env["TRIGGER_PROM_QUERY"].value)
	assert.Equal(t, "600", env["TRIGGERS_TIMEOUT"].value)
	assert.Equal(t, "10", env["TRIGGERS_INTERVAL"].value)
	assert.Equal(t, "all_of", env["TRIGGERS_MODE"].value)
	assert.Equal(t, "run_anyway", env["TRIGGERS_ON_TIMEOUT"].value)
	assert.Equal(t, "tok", env["PROMETHEUS_BEARER_TOKEN"].value)
	assert.True(t, env["PROMETHEUS_BEARER_TOKEN"].secret)

	// Without trigger args and skipDefault=true, no trigger env vars are set.
	environment, _, err = ParseFlags(scenario, []string{"node-cpu-hog"}, collected, true)
	assert.Nil(t, err)
	_, hasQuery := (*environment)["TRIGGER_PROM_QUERY"]
	assert.False(t, hasQuery)
	_, hasCommand := (*environment)["TRIGGER_COMMAND"]
	assert.False(t, hasCommand)
}
