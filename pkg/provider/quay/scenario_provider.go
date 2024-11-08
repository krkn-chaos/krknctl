package quay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
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

func (p *ScenarioProvider) GetScenarios(dataSource string) (*[]models.ScenarioTag, error) {
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
	defer resp.Body.Close()
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

	return &scenarioTags, nil
}

func (p *ScenarioProvider) getInstructionScenario(rootNodeName string) container_manager.ScenarioNode {
	node := container_manager.ScenarioNode{}
	node.Comment = fmt.Sprintf("**READ CAREFULLY** To create your scenario run plan, assign an ID to each scenario definition (or keep the existing randomly assigned ones if preferred). "+
		"Define dependencies between scenarios using the `depends_on` field, ensuring there are no cycles (including transitive ones) "+
		"or self-references.Nodes not referenced will not be executed, Nodes without dependencies will run first, "+
		"while nodes that share the same parent will execute in parallel. [CURRENT ROOT SCENARIO IS `%s`]", rootNodeName)
	return node
}

func (p *ScenarioProvider) ScaffoldScenarios(scenarios []string, dataSource string) (*string, error) {
	var scenarioDetails []models.ScenarioDetail
	babbler := babble.NewBabbler()
	babbler.Count = 1
	for _, scenarioName := range scenarios {
		scenarioDetail, err := p.GetScenarioDetail(scenarioName, dataSource)
		if err != nil {
			// skips images that doesn't have proper scenario details in the Containerfile
			continue
		}
		if scenarioDetail == nil {
			return nil, fmt.Errorf("scenario %s does not exist", scenarioName)
		}
		scenarioDetails = append(scenarioDetails, *scenarioDetail)
	}
	// builds all the indexes for the json upfront so I can suggest the root node in the _comment
	var indexes []string
	for _, scenario := range scenarios {
		indexes = append(indexes, fmt.Sprintf("%s-%s", scenario, strings.ToLower(babbler.Babble())))
	}
	var scenarioNodes = make(map[string]container_manager.ScenarioNode)

	scenarioNodes["_comment"] = p.getInstructionScenario(indexes[0])
	for i, scenarioDetail := range scenarioDetails {
		indexes = append(indexes, strings.ToLower(babbler.Babble()))

		scenarioNode := container_manager.ScenarioNode{}
		if i > 0 {
			scenarioNode.Parent = &indexes[i-1]
		}
		scenarioNode.Image = p.Config.GetQuayImageUri() + ":" + scenarioDetail.Name
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

func (p *ScenarioProvider) GetScenarioDetail(scenario string, dataSource string) (*models.ScenarioDetail, error) {
	scenarios, err := p.GetScenarios(dataSource)
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

	baseURL, err := url.Parse(dataSource + "/manifest/" + foundScenario.Digest)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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

	titleLabel := manifest.GetKrknctlLabel("krknctl.title")
	descriptionLabel := manifest.GetKrknctlLabel("krknctl.description")
	inputFieldsLabel := manifest.GetKrknctlLabel("krknctl.input_fields")

	if titleLabel == nil {
		return nil, fmt.Errorf("krknctl.title LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
	}
	if descriptionLabel == nil {
		return nil, fmt.Errorf("krknctl.description LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
	}
	if inputFieldsLabel == nil {
		return nil, fmt.Errorf("krknctl.input_fields LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
	}

	parsedTitle, err := p.parseTitle(*titleLabel)
	if err != nil {
		return nil, err
	}
	parsedDescription, err := p.parseDescription(*descriptionLabel)
	if err != nil {
		return nil, err
	}

	parsedInputFields, err := p.parseInputFields(*inputFieldsLabel)
	if err != nil {
		return nil, err
	}

	scenarioDetail.Title = *parsedTitle
	scenarioDetail.Description = *parsedDescription
	scenarioDetail.Fields = parsedInputFields
	return &scenarioDetail, nil
}

func (p *ScenarioProvider) parseTitle(s string) (*string, error) {
	reDoubleQuotes, err := regexp.Compile("LABEL krknctl\\.title=\"?(.*)\"?")
	if err != nil {
		return nil, err
	}
	matches := reDoubleQuotes.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("title not found in image manifest")
	}
	return &matches[1], nil
}

func (p *ScenarioProvider) parseDescription(s string) (*string, error) {
	re, err := regexp.Compile("LABEL krknctl\\.description=\"?(.*)\"?")
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("description not found in image manifest")
	}
	return &matches[1], nil
}

func (p *ScenarioProvider) parseKubeconfigPath(s string) (*string, error) {
	re, err := regexp.Compile("LABEL krknctl\\.kubeconfig_path=\"?(.*)\"?")
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("kubeconfig_path not found in image manifest")
	}
	return &matches[1], nil
}

func (p *ScenarioProvider) parseInputFields(s string) ([]typing.InputField, error) {
	re, err := regexp.Compile("LABEL krknctl\\.input_fields=\\'?(\\[.*\\])\\'?")
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
