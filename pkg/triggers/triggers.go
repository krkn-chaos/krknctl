// Package triggers documents the Hub metadata contract for chaos triggers
// (command, prometheus, http, k8s, etc.) and provides helpers for trigger detection.
//
// Flag discovery is driven entirely and dynamically by scenario/global OCI labels
// from krkn-hub `krknctl-input.json`. When trigger fields are present, the existing
// dynamic flag system in `cmd/run.go` exposes them and ParseFlags maps each flag
// to its `variable` env var for the scenario container.
package triggers

import (
	"strings"

	"github.com/krkn-chaos/krknctl/pkg/typing"
)

// GroupTriggers is the schema group name for chaos triggers in krknctl-input.json.
const GroupTriggers = "triggers"

// SupportsTriggers reports whether the given input fields declare chaos trigger
// configuration based on the triggers schema group and field naming.
func SupportsTriggers(fields []typing.InputField) bool {
	for _, field := range fields {
		if field.Group != nil && *field.Group == GroupTriggers {
			return true
		}
		if field.Name != nil {
			if *field.Name == GroupTriggers && field.Type == typing.Group {
				return true
			}
			if strings.HasPrefix(*field.Name, "trigger-") || strings.HasPrefix(*field.Name, "triggers-") {
				return true
			}
		}
	}
	return false
}
