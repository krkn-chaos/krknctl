// Package triggers documents the Hub metadata contract for Prometheus
// (and related) chaos triggers, and helpers used by CLI tests.
//
// Flag discovery is driven entirely by scenario/global OCI labels from
// krkn-hub `krknctl-input.json`. When those fields are present, the existing
// dynamic flag system in `cmd/run.go` exposes them and ParseFlags maps each
// flag to its `variable` env var for the scenario container. YAML injection
// into config.yaml is owned by krkn-hub (Phase 2), not krknctl.
package triggers

import "github.com/krkn-chaos/krknctl/pkg/typing"

// Well-known CLI flag names for Prometheus / shared trigger configuration.
// These must match Hub schema field `name` values (Phase 2).
const (
	FlagTriggerPromQuery         = "trigger-prom-query"
	FlagTriggersTimeout          = "triggers-timeout"
	FlagTriggersInterval         = "triggers-interval"
	FlagTriggersMode             = "triggers-mode"
	FlagTriggersOnTimeout        = "triggers-on-timeout"
	FlagPrometheusURL            = "prometheus-url"
	FlagPrometheusBearerToken    = "prometheus-bearer-token"
)

// Environment variable names passed into the scenario container.
const (
	EnvTriggerPromQuery      = "TRIGGER_PROM_QUERY"
	EnvTriggersTimeout       = "TRIGGERS_TIMEOUT"
	EnvTriggersInterval      = "TRIGGERS_INTERVAL"
	EnvTriggersMode          = "TRIGGERS_MODE"
	EnvTriggersOnTimeout     = "TRIGGERS_ON_TIMEOUT"
	EnvPrometheusURL         = "PROMETHEUS_URL"
	EnvPrometheusBearerToken = "PROMETHEUS_BEARER_TOKEN"
)

// FlagToEnv maps trigger-related CLI flag names to container env vars.
var FlagToEnv = map[string]string{
	FlagTriggerPromQuery:      EnvTriggerPromQuery,
	FlagTriggersTimeout:       EnvTriggersTimeout,
	FlagTriggersInterval:      EnvTriggersInterval,
	FlagTriggersMode:          EnvTriggersMode,
	FlagTriggersOnTimeout:     EnvTriggersOnTimeout,
	FlagPrometheusURL:         EnvPrometheusURL,
	FlagPrometheusBearerToken: EnvPrometheusBearerToken,
}

// KnownTriggerFlagNames returns the trigger-specific CLI flag names used to
// detect whether a scenario declares trigger support. prometheus-url is
// excluded here because it already exists as a general performance-monitoring
// field on many images and would false-positive "trigger support".
func KnownTriggerFlagNames() []string {
	return []string{
		FlagTriggerPromQuery,
		FlagTriggersTimeout,
		FlagTriggersInterval,
		FlagTriggersMode,
		FlagTriggersOnTimeout,
	}
}

// SupportsTriggers reports whether the given input fields declare Prometheus
// trigger configuration (so krknctl will expose those flags for the scenario).
func SupportsTriggers(fields []typing.InputField) bool {
	known := make(map[string]struct{}, len(KnownTriggerFlagNames()))
	for _, name := range KnownTriggerFlagNames() {
		known[name] = struct{}{}
	}
	for _, field := range fields {
		if field.Name == nil {
			continue
		}
		if _, ok := known[*field.Name]; ok {
			return true
		}
	}
	return false
}
