package triggers

import (
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestSupportsTriggers(t *testing.T) {
	noTriggers := []typing.InputField{
		{Name: strPtr("duration"), Variable: strPtr("END"), Type: typing.Number},
		// prometheus-url alone is performance monitoring, not trigger support
		{Name: strPtr(FlagPrometheusURL), Variable: strPtr(EnvPrometheusURL), Type: typing.String},
	}
	assert.False(t, SupportsTriggers(noTriggers))
	assert.False(t, SupportsTriggers(nil))

	withProm := []typing.InputField{
		{Name: strPtr("duration"), Variable: strPtr("END"), Type: typing.Number},
		{Name: strPtr(FlagTriggerPromQuery), Variable: strPtr(EnvTriggerPromQuery), Type: typing.String},
	}
	assert.True(t, SupportsTriggers(withProm))

	withTimeoutOnly := []typing.InputField{
		{Name: strPtr(FlagTriggersTimeout), Variable: strPtr(EnvTriggersTimeout), Type: typing.Number},
	}
	assert.True(t, SupportsTriggers(withTimeoutOnly))
}

func TestFlagToEnvContract(t *testing.T) {
	assert.Equal(t, EnvTriggerPromQuery, FlagToEnv[FlagTriggerPromQuery])
	assert.Equal(t, EnvTriggersTimeout, FlagToEnv[FlagTriggersTimeout])
	assert.Equal(t, EnvTriggersInterval, FlagToEnv[FlagTriggersInterval])
	assert.Equal(t, EnvTriggersMode, FlagToEnv[FlagTriggersMode])
	assert.Equal(t, EnvTriggersOnTimeout, FlagToEnv[FlagTriggersOnTimeout])
	assert.Equal(t, EnvPrometheusURL, FlagToEnv[FlagPrometheusURL])
	assert.Equal(t, EnvPrometheusBearerToken, FlagToEnv[FlagPrometheusBearerToken])
}

func TestKnownTriggerFlagNames(t *testing.T) {
	names := KnownTriggerFlagNames()
	assert.Contains(t, names, FlagTriggerPromQuery)
	assert.Contains(t, names, FlagTriggersTimeout)
	assert.Contains(t, names, FlagTriggersInterval)
	assert.NotContains(t, names, FlagPrometheusURL,
		"prometheus-url must not alone imply trigger support")
}
