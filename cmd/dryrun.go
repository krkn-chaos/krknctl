package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
)

const dryRunClientValue = "client"

// DryRunResult holds the outcome of a local validation pass.
type DryRunResult struct {
	Valid    bool
	Messages []dryRunMessage
}

type dryRunMessage struct {
	ok   bool
	text string
}

func (r *DryRunResult) addOK(msg string) {
	r.Messages = append(r.Messages, dryRunMessage{ok: true, text: msg})
}

func (r *DryRunResult) addFail(msg string) {
	r.Valid = false
	r.Messages = append(r.Messages, dryRunMessage{ok: false, text: msg})
}

// Print writes the validation results to stdout using the project colour scheme.
func (r *DryRunResult) Print() {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgHiRed).SprintFunc()
	for _, m := range r.Messages {
		if m.ok {
			fmt.Printf("%s %s\n", green("✔"), m.text)
		} else {
			fmt.Printf("%s %s\n", red("✖"), m.text)
		}
	}
}

// parseDryRunFlag extracts the --dry-run flag value from raw args.
// Returns ("", false, nil) when the flag is absent.
// Returns an error when the flag is present but has an unsupported value.
func parseDryRunFlag(args []string) (string, bool, error) {
	value, found, err := ParseArgValue(args, "--dry-run")
	if err != nil {
		return "", false, fmt.Errorf("--dry-run: %w", err)
	}
	if !found {
		return "", false, nil
	}
	if strings.ToLower(value) != dryRunClientValue {
		return "", false, fmt.Errorf("--dry-run only accepts %q, got %q", dryRunClientValue, value)
	}
	return value, true, nil
}

// validateScenarioLocally performs client-side validation of a scenario's
// input fields against the values supplied in args.
// It never contacts the cluster, the container runtime, or the image registry.
//
// scenarioDetail and globalDetail may be nil (e.g. when the registry is
// unreachable in dry-run mode); in that case only the args themselves are
// checked for well-formedness.
func validateScenarioLocally(
	scenarioDetail *models.ScenarioDetail,
	globalDetail *models.ScenarioDetail,
	args []string,
) *DryRunResult {
	result := &DryRunResult{Valid: true}

	// ── 1. schema present ────────────────────────────────────────────────────
	if scenarioDetail == nil {
		result.addFail("scenario schema not found")
		return result
	}
	result.addOK("Scenario schema valid")

	// Build a single flat slice of all fields without mutating the originals.
	// Using append on a nil/empty base avoids the capacity-aliasing bug where
	// append(scenarioDetail.Fields, ...) could overwrite scenarioDetail.Fields.
	var allFields []typing.InputField
	allFields = append(allFields, scenarioDetail.Fields...)
	if globalDetail != nil {
		allFields = append(allFields, globalDetail.Fields...)
	}

	// ── 2. required fields present ───────────────────────────────────────────
	missingRequired := false
	for _, field := range allFields {
		if !field.Required {
			continue
		}
		flagName := fmt.Sprintf("--%s", *field.Name)
		_, found, _ := ParseArgValue(args, flagName)
		hasDefault := field.Default != nil && *field.Default != ""
		if !found && !hasDefault {
			result.addFail(fmt.Sprintf("Missing required field: %s", *field.Name))
			missingRequired = true
		}
	}
	if !missingRequired {
		result.addOK("All required fields present")
	}

	// ── 3. validate each supplied value ──────────────────────────────────────
	validationFailed := false
	for _, field := range allFields {
		flagName := fmt.Sprintf("--%s", *field.Name)
		rawValue, found, err := ParseArgValue(args, flagName)
		if err != nil {
			result.addFail(fmt.Sprintf("Flag parse error for %s: %v", *field.Name, err))
			validationFailed = true
			continue
		}
		if !found {
			continue // not supplied — required check already handled above
		}
		if _, err := field.Validate(&rawValue); err != nil {
			result.addFail(fmt.Sprintf("Invalid value for %s (%s): %v", *field.Name, field.Type.String(), err))
			validationFailed = true
		}
	}
	if !validationFailed {
		result.addOK("Values validated")
	}

	return result
}
