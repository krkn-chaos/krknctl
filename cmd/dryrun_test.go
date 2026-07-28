package cmd

import (
	"encoding/json"
	"testing"

	providerModels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/stretchr/testify/assert"
)

// 🤖 Assisted with Claude Code (claude.ai/code)

func unmarshalField(t *testing.T, raw string) typing.InputField {
	var field typing.InputField
	err := json.Unmarshal([]byte(raw), &field)
	assert.Nil(t, err)
	return field
}

func TestValidateScenarioConfig(t *testing.T) {
	requiredNoDefault := unmarshalField(t, `
	{
		"name":"namespace",
		"short_description":"Namespace",
		"description":"Namespace",
		"variable":"NAMESPACE",
		"type":"string",
		"required":"true"
	}`)

	requiredWithDefault := unmarshalField(t, `
	{
		"name":"disruption-count",
		"short_description":"Disruption count",
		"description":"Disruption count",
		"variable":"DISRUPTION_COUNT",
		"type":"number",
		"required":"true",
		"default":"1"
	}`)

	optionalNumber := unmarshalField(t, `
	{
		"name":"kill-timeout",
		"short_description":"Kill timeout",
		"description":"Kill timeout",
		"variable":"KILL_TIMEOUT",
		"type":"number",
		"default":"180"
	}`)

	trafficType := unmarshalField(t, `
	{
		"name":"traffic-type",
		"short_description":"Traffic Type",
		"description":"Traffic Type",
		"variable":"TRAFFIC_TYPE",
		"type":"enum",
		"allowed_values":"ingress,egress",
		"separator":",",
		"required":"true"
	}`)

	requiredGlobal := unmarshalField(t, `
	{
		"name":"kubeconfig",
		"short_description":"kubeconfig",
		"description":"kubeconfig",
		"variable":"KUBECONFIG",
		"type":"string",
		"required":"true"
	}`)

	requiredGlobalNumber := unmarshalField(t, `
	{
		"name":"wait-duration",
		"short_description":"wait duration",
		"description":"wait duration",
		"variable":"WAIT_DURATION",
		"type":"number",
		"required":"true"
	}`)

	// happy path: all required fields provided
	scenario := &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredNoDefault, trafficType},
	}
	args := []string{"--namespace", "dev", "--traffic-type", "egress"}
	valid, errs := ValidateScenarioConfig(scenario, nil, args)
	assert.True(t, valid)
	assert.Empty(t, errs)

	// missing required field
	scenario = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredNoDefault},
	}
	valid, errs = ValidateScenarioConfig(scenario, nil, []string{})
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	// required field filled by schema default
	scenario = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredWithDefault},
	}
	valid, errs = ValidateScenarioConfig(scenario, nil, []string{})
	assert.True(t, valid)
	assert.Empty(t, errs)

	// invalid number value
	scenario = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{optionalNumber},
	}
	args = []string{"--kill-timeout", "notanumber"}
	valid, errs = ValidateScenarioConfig(scenario, nil, args)
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	// invalid enum value
	scenario = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{trafficType},
	}
	args = []string{"--traffic-type", "not-a-real-choice"}
	valid, errs = ValidateScenarioConfig(scenario, nil, args)
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	// explicitly empty string satisfies a required String field: Validate
	// only treats a nil pointer as missing for Type==String
	// (pkg/typing/field.go), and dry-run intentionally mirrors that
	// exact behavior rather than adding a stricter check of its own, so
	// it stays a faithful predictor of the real run path
	global := &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredGlobal},
	}
	args = []string{"--kubeconfig", ""}
	valid, errs = ValidateScenarioConfig(nil, global, args)
	assert.True(t, valid)
	assert.Empty(t, errs)

	// explicitly empty value fails a required non-String global field
	global = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredGlobalNumber},
	}
	args = []string{"--wait-duration", ""}
	valid, errs = ValidateScenarioConfig(nil, global, args)
	assert.False(t, valid)
	assert.Len(t, errs, 1)

	// required global field not passed at all is skipped, mirroring
	// ParseFlags' skipDefault=true behavior for globals on the real run
	// path
	global = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredGlobal},
	}
	valid, errs = ValidateScenarioConfig(nil, global, []string{})
	assert.True(t, valid)
	assert.Empty(t, errs)

	// multiple independent errors collected in one call
	scenario = &providerModels.ScenarioDetail{
		Fields: []typing.InputField{requiredNoDefault, trafficType},
	}
	args = []string{"--traffic-type", "not-a-real-choice"}
	valid, errs = ValidateScenarioConfig(scenario, nil, args)
	assert.False(t, valid)
	assert.Len(t, errs, 2)
}

func TestPrintDryRunResult(t *testing.T) {
	// valid result prints without error
	err := PrintDryRunResult(true, nil)
	assert.Nil(t, err)

	// invalid result prints without error
	err = PrintDryRunResult(false, []string{"missing required field: namespace"})
	assert.Nil(t, err)
}