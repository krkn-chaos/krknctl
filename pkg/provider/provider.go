package provider

import (
	"encoding/json"
	"errors"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"regexp"
)

type Mode int64

const (
	Quay = iota
	Private
)

type BaseScenarioProvider struct {
	Config config.Config
}

func (p *BaseScenarioProvider) ParseTitle(s string, isGlobalEnvironment bool) (*string, error) {
	var regex = ""
	if isGlobalEnvironment == true {
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
	var regex string = ""
	if isGlobalEnvironment == true {
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

func (p *BaseScenarioProvider) ParseInputFields(s string, isGlobalEnvironment bool) ([]typing.InputField, error) {
	var regex string = ""
	if isGlobalEnvironment == true {
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
	GetGlobalEnvironment(registry *models.RegistryV2) (*models.ScenarioDetail, error)
	GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error)
	ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2) (*string, error)
}

type ContainerLayer interface {
	GetCommands() []string
}
