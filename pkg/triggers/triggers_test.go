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
		// prometheus-url alone in the prometheus group is performance monitoring, not trigger support
		{Name: strPtr("prometheus-url"), Variable: strPtr("PROMETHEUS_URL"), Type: typing.String, Group: strPtr("prometheus")},
	}
	assert.False(t, SupportsTriggers(noTriggers))
	assert.False(t, SupportsTriggers(nil))

	// Prometheus trigger query
	withProm := []typing.InputField{
		{Name: strPtr("duration"), Variable: strPtr("END"), Type: typing.Number},
		{Name: strPtr("trigger-prom-query"), Variable: strPtr("TRIGGER_PROM_QUERY"), Type: typing.String, Group: strPtr(GroupTriggers)},
	}
	assert.True(t, SupportsTriggers(withProm))

	// Command trigger
	withCommand := []typing.InputField{
		{
			Name:        strPtr("trigger-command"),
			Description: strPtr("Shell command to evaluate before chaos starts"),
			Variable:    strPtr("TRIGGER_COMMAND"),
			Type:        typing.String,
			Group:       strPtr(GroupTriggers),
		},
	}
	assert.True(t, SupportsTriggers(withCommand))

	// Shared trigger timing field
	withTimeoutOnly := []typing.InputField{
		{Name: strPtr("triggers-timeout"), Variable: strPtr("TRIGGERS_TIMEOUT"), Type: typing.Number, Group: strPtr(GroupTriggers)},
	}
	assert.True(t, SupportsTriggers(withTimeoutOnly))

	// Triggers group metadata field
	withGroupField := []typing.InputField{
		{Name: strPtr(GroupTriggers), Type: typing.Group},
	}
	assert.True(t, SupportsTriggers(withGroupField))
}
