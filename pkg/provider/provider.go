// Package provider defines the interface for the tool metadata provider from the various data sources available
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"regexp"
	"strconv"
)

type Mode int64

const (
	Quay = iota
	Private
)

type BaseScenarioProvider struct {
	Config config.Config
	Cache  cache.Cache
}

func (p *BaseScenarioProvider) ParseTitle(s string, isGlobalEnvironment bool) (*string, error) {
	var regex = ""
	if isGlobalEnvironment {
		regex = p.Config.LabelTitleRegexGlobal
	} else {
		regex = p.Config.LabelTitleRegex
	}
	reDoubleQuotes, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	matches := reDoubleQuotes.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("title not found in image manifest")
	}
	if len(matches) < 2 {
		return nil, errors.New("title not found in image manifest")
	}
	return &matches[1], nil
}

func (p *BaseScenarioProvider) ParseDescription(s string, isGlobalEnvironment bool) (*string, error) {
	var regex = ""
	if isGlobalEnvironment {
		regex = p.Config.LabelDescriptionRegexGlobal
	} else {
		regex = p.Config.LabelDescriptionRegex
	}
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("description not found in image manifest")
	}
	return &matches[1], nil
}
func (p *BaseScenarioProvider) ParseIsAScenario(s string) (*bool,
	error) {
	return parseBoolLabel(s, p.Config.LabelIsAScenarioRegex, "is_a_scenario")
}

func (p *BaseScenarioProvider) ParseHasRollback(s string) (*bool, error) {
	return parseBoolLabel(s, p.Config.LabelHasRollbackRegex, "has_rollback")
}

func parseBoolLabel(s string, regex string, labelName string) (*bool, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("label %s not found in image manifest (input: %q, regex: %q)", labelName, s, regex)
	}
	if len(matches) < 2 {
		return nil, fmt.Errorf("label %s value does not match expected format (input: %q, regex: %q, matches: %v)", labelName, s, regex, matches)
	}
	boolValue, err := strconv.ParseBool(matches[1])
	if err != nil {
		return nil, fmt.Errorf("label %s has invalid boolean value %q: %w", labelName, matches[1], err)
	}
	return &boolValue, nil
}

// PopulateBooleanLabels parses is_a_scenario and has_rollback labels from container layers
// and sets them on the ScenarioDetail. Only applies to non-global environments.
func (p *BaseScenarioProvider) PopulateBooleanLabels(detail *models.ScenarioDetail, layers []ContainerLayer, isGlobalEnvironment bool) error {
	if detail == nil {
		return errors.New("scenario detail cannot be nil")
	}
	if isGlobalEnvironment {
		return nil
	}

	foundIsAScenario := GetKrknctlLabel(p.Config.LabelIsAScenario, layers)
	if foundIsAScenario != nil {
		parsed, err := p.ParseIsAScenario(*foundIsAScenario)
		if err != nil {
			return err
		}
		detail.IsAScenario = *parsed
	} else {
		detail.IsAScenario = false
	}

	foundHasRollback := GetKrknctlLabel(p.Config.LabelHasRollback, layers)
	if foundHasRollback != nil {
		parsed, err := p.ParseHasRollback(*foundHasRollback)
		if err != nil {
			return err
		}
		detail.HasRollback = *parsed
	} else {
		detail.HasRollback = false
	}

	return nil
}

func (p *BaseScenarioProvider) ParseInputFields(s string, isGlobalEnvironment bool) ([]typing.InputField, error) {
	var regex = ""
	if isGlobalEnvironment {
		regex = p.Config.LabelInputFieldsRegexGlobal
	} else {
		regex = p.Config.LabelInputFieldsRegex
	}
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	var fields []typing.InputField
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("input_fields not found in image manifest")
	}
	err = json.Unmarshal([]byte(matches[1]), &fields)
	if err != nil {
		return nil, err
	}
	return fields, nil
}

type ScenarioDataProvider interface {
	GetRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error)
	GetGlobalEnvironment(registry *models.RegistryV2, scenario string) (*models.ScenarioDetail, error)
	GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error)
	// GetImageSignatureStatus reports the cosign signature state of a single
	// scenario image. It is the opt-in counterpart to GetRegistryImages: callers
	// that want the SignatureStatus invoke it per tag (controlling their own
	// concurrency), while the plain listing path stays free of verification
	// round-trips. It fails safe — verification outcomes are folded into the
	// returned status (never signed unless a trusted signature verifies), and a
	// non-nil error is returned only for setup failures (e.g. the image
	// reference could not be constructed), in which case the status is unknown.
	GetImageSignatureStatus(ctx context.Context, registry *models.RegistryV2, tag models.ScenarioTag) (verify.SignatureStatus, error)
	ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, random bool, seed *ScaffoldSeed) (*string, error)
}

// ImageReference builds a fully-qualified image reference for a scenario tag.
// It pins to the immutable digest when the tag carries one (anti-TOCTOU and one
// fewer registry round-trip, since the verifier does not need to re-resolve the
// tag), and falls back to the mutable tag name otherwise.
func ImageReference(base string, tag models.ScenarioTag) string {
	if tag.Digest != nil && *tag.Digest != "" {
		return fmt.Sprintf("%s@%s", base, *tag.Digest)
	}
	return fmt.Sprintf("%s:%s", base, tag.Name)
}

// ImageSignatureStatus verifies ref with opts and returns its SignatureStatus,
// caching definitive results by reference so repeated lookups (and tags that
// share a digest, e.g. latest == vX.Y) do not re-hit the registry. Transient
// "unknown" results are never cached: they reflect an outage/timeout that must
// be re-evaluated on the next request. This is shared by every provider so the
// caching and verification behaviour is identical regardless of data source.
func (p *BaseScenarioProvider) ImageSignatureStatus(ctx context.Context, ref string, opts verify.Options) verify.SignatureStatus {
	cacheKey := "sigstatus:" + ref
	if cached := p.Cache.GetString(cacheKey); cached != nil {
		return verify.SignatureStatus(*cached)
	}
	status := verify.StatusFor(ctx, ref, opts)
	if status != verify.SignatureUnknown {
		p.Cache.SetString(cacheKey, string(status))
	}
	return status
}

type ContainerLayer interface {
	GetCommands() []string
}

type ScaffoldSeed struct {
	Path              string `json:"path"`
	NumberOfScenarios int    `json:"number_of_scenarios"`
}
