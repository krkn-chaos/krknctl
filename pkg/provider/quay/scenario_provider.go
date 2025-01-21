package quay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	models2 "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/tjarratt/babble"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type ScenarioProvider struct {
	Config *config.Config
}

func (p *ScenarioProvider) getRegistryImages(dataSource string) (*[]models.ScenarioTag, error) {
	tagBaseUrl, err := url.Parse(dataSource + "/tag")
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Add("onlyActiveTags", "true")
	params.Add("limit", "100")
	// currently paging support is not needed
	params.Add("page", "1")
	tagBaseUrl.RawQuery = params.Encode()

	resp, _ := http.Get(tagBaseUrl.String())
	var deferErr error = nil
	defer func() {
		deferErr = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve tags, " + tagBaseUrl.String() + " returned: " + resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	var quayPage TagPage
	err = json.Unmarshal(bodyBytes, &quayPage)

	var scenarioTags []models.ScenarioTag
	for _, tag := range quayPage.Tags {
		scenarioTags = append(scenarioTags, models.ScenarioTag{
			Name:         tag.Name,
			LastModified: tag.LastModified,
			Size:         tag.Size,
			Digest:       tag.ManifestDigest,
		})
	}

	return &scenarioTags, deferErr
}

func (p *ScenarioProvider) GetRegistryImages() (*[]models.ScenarioTag, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryApiUri()
	if err != nil {
		return nil, err
	}

	scenarioTags, err := p.getRegistryImages(dataSource)
	if err != nil {
		return nil, err
	}
	return scenarioTags, nil
}

func (p *ScenarioProvider) getInstructionScenario(rootNodeName string) models2.ScenarioNode {
	node := models2.ScenarioNode{}
	node.Comment = fmt.Sprintf("**READ CAREFULLY** To create your scenario run plan, assign an ID to each scenario definition (or keep the existing randomly assigned ones if preferred). "+
		"Define dependencies between scenarios using the `depends_on` field, ensuring there are no cycles (including transitive ones) "+
		"or self-references.Nodes not referenced will not be executed, Nodes without dependencies will run first, "+
		"while nodes that share the same parent will execute in parallel. [CURRENT ROOT SCENARIO IS `%s`]", rootNodeName)
	return node
}

func (p *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool) (*string, error) {
	var scenarioDetails []models.ScenarioDetail

	// handles babble panic when american word dictionary is not installed
	defer func() {
		if err := recover(); err != nil {
			_, err = color.New(color.FgRed, color.Underline).Println("impossible to locate american words dictionary " +
				"please refer to the documentation on how to install this dependency https://github.com/krkn-chaos/krknctl#Requirements")
		}
	}()
	babbler := babble.NewBabbler()
	babbler.Count = 1
	for _, scenarioName := range scenarios {
		scenarioDetail, err := p.GetScenarioDetail(scenarioName)

		if err != nil {
			return nil, err
		}

		if scenarioDetail == nil {
			return nil, fmt.Errorf("scenario '%s' not found", scenarioName)
		}

		scenarioDetails = append(scenarioDetails, *scenarioDetail)
	}
	// builds all the indexes for the json upfront, so I can suggest the root node in the _comment
	var indexes []string
	for _, scenario := range scenarios {
		indexes = append(indexes, fmt.Sprintf("%s-%s", scenario, strings.ToLower(babbler.Babble())))
	}
	var scenarioNodes = make(map[string]models2.ScenarioNode)

	scenarioNodes["_comment"] = p.getInstructionScenario(indexes[0])
	for i, scenarioDetail := range scenarioDetails {
		indexes = append(indexes, strings.ToLower(babbler.Babble()))

		scenarioNode := models2.ScenarioNode{}
		if i > 0 {
			scenarioNode.Parent = &indexes[i-1]
		} else {
			scenarioNode.Comment = "I'm the root Node!"
		}
		imageUri, err := p.Config.GetQuayImageUri()
		if err != nil {
			return nil, err
		}
		scenarioNode.Image = imageUri + ":" + scenarioDetail.Name
		scenarioNode.Name = scenarioDetail.Name
		scenarioNode.Env = make(map[string]string)
		scenarioNode.Volumes = make(map[string]string)

		for _, detail := range scenarioDetail.Fields {

			switch detail.Type {
			case typing.File:
				scenarioNode.Volumes[fmt.Sprintf("<put your local file_%d path here>", i)] = *detail.MountPath
			default:
				if detail.Default != nil && *detail.Default != "" {
					scenarioNode.Env[*detail.Variable] = *detail.Default
				} else {
					var required = ""
					if detail.Required == true {
						required = "(required)"
					}
					scenarioNode.Env[*detail.Variable] = fmt.Sprintf("<%s%s>", *detail.Description, required)
				}

			}

		}

		if includeGlobalEnv == true {
			globalDetail, err := p.GetGlobalEnvironment()
			if err != nil {
				return nil, err
			}

			for _, field := range globalDetail.Fields {
				scenarioNode.Env[*field.Variable] = *field.Default

			}
		}
		scenarioNodes[indexes[i]] = scenarioNode
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(scenarioNodes)
	if err != nil {
		return nil, err
	}
	jsonBuf := buf.String()
	return &jsonBuf, nil
}

func (p *ScenarioProvider) getScenarioDetail(dataSource string, foundScenario *models.ScenarioTag, isGlobalEnvironment bool) (*models.ScenarioDetail, error) {
	baseURL, err := url.Parse(dataSource + "/manifest/" + (*foundScenario).Digest)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return nil, err
	}

	var deferErr error = nil
	defer func() {
		deferErr = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve scenario details, " + baseURL.String() + " returned: " + resp.Status)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	err = json.Unmarshal(bodyBytes, &manifest)
	if err != nil {
		return nil, err
	}

	scenarioDetail := models.ScenarioDetail{
		ScenarioTag: *foundScenario,
	}
	var titleLabel *string = nil
	var descriptionLabel *string = nil
	var inputFieldsLabel *string = nil
	if isGlobalEnvironment == true {
		titleLabel = manifest.GetKrknctlLabel(p.Config.LabelTitleGlobal)
		descriptionLabel = manifest.GetKrknctlLabel(p.Config.LabelDescriptionGlobal)
		inputFieldsLabel = manifest.GetKrknctlLabel(p.Config.LabelInputFieldsGlobal)
	} else {
		titleLabel = manifest.GetKrknctlLabel(p.Config.LabelTitle)
		descriptionLabel = manifest.GetKrknctlLabel(p.Config.LabelDescription)
		inputFieldsLabel = manifest.GetKrknctlLabel(p.Config.LabelInputFields)
	}

	if titleLabel == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", p.Config.LabelTitle, foundScenario.Name, foundScenario.Digest)
	}
	if descriptionLabel == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", p.Config.LabelDescription, foundScenario.Name, foundScenario.Digest)
	}
	if inputFieldsLabel == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", p.Config.LabelInputFields, foundScenario.Name, foundScenario.Digest)
	}

	parsedTitle, err := p.parseTitle(*titleLabel, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}
	parsedDescription, err := p.parseDescription(*descriptionLabel, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	parsedInputFields, err := p.parseInputFields(*inputFieldsLabel, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	scenarioDetail.Title = *parsedTitle
	scenarioDetail.Description = *parsedDescription
	scenarioDetail.Fields = parsedInputFields
	return &scenarioDetail, deferErr
}

func (p *ScenarioProvider) GetScenarioDetail(scenario string) (*models.ScenarioDetail, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryApiUri()
	if err != nil {
		return nil, err
	}
	scenarios, err := p.GetRegistryImages()
	if err != nil {
		return nil, err
	}
	var foundScenario *models.ScenarioTag = nil
	for _, scenarioTag := range *scenarios {
		if scenarioTag.Name == scenario {
			foundScenario = &scenarioTag
		}
	}
	if foundScenario == nil {
		return nil, nil
	}

	scenarioDetail, err := p.getScenarioDetail(dataSource, foundScenario, false)
	if err != nil {
		return nil, err
	}
	return scenarioDetail, nil
}

func (p *ScenarioProvider) GetGlobalEnvironment() (*models.ScenarioDetail, error) {
	dataSource, err := p.Config.GetQuayEnvironmentApiUri()
	if err != nil {
		return nil, err
	}
	var foundScenario *models.ScenarioTag = nil
	scenarios, err := p.getRegistryImages(dataSource)
	if err != nil {
		return nil, err
	}
	if scenarios == nil {
		return nil, fmt.Errorf("no tags found in registry %s", dataSource)
	}
	for _, scenarioTag := range *scenarios {
		if scenarioTag.Name == p.Config.QuayBaseImageTag {
			foundScenario = &scenarioTag
		}
	}
	if foundScenario == nil {
		return nil, nil
	}

	globalEnvDetail, err := p.getScenarioDetail(dataSource, foundScenario, true)
	if err != nil {
		return nil, err
	}
	return globalEnvDetail, nil

}

func (p *ScenarioProvider) parseTitle(s string, isGlobalEnvironment bool) (*string, error) {
	var regex string = ""
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
	return &matches[1], nil
}

func (p *ScenarioProvider) parseDescription(s string, isGlobalEnvironment bool) (*string, error) {
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

func (p *ScenarioProvider) parseInputFields(s string, isGlobalEnvironment bool) ([]typing.InputField, error) {
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
