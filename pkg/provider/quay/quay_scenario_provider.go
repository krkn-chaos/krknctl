package quay

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"regexp"

	"io"
	"log"
	"net/http"
	"net/url"
)

type ScenarioProvider struct {
	Config *config.Config
}

func (p *ScenarioProvider) GetScenarios() (*[]models.ScenarioTag, error) {
	baseURL, _ := url.Parse(p.Config.QuayApi + "/" + p.Config.QuayRegistry + "/tag")
	params := url.Values{}
	params.Add("onlyActiveTags", "true")
	params.Add("limit", "100")
	// currently paging support is not needed
	params.Add("page", "1")
	baseURL.RawQuery = params.Encode()

	resp, _ := http.Get(baseURL.String())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve tags, " + baseURL.String() + " returned: " + resp.Status)
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

func (p *ScenarioProvider) GetScenarioDetail(scenario string) (*models.ScenarioDetail, error) {
	scenarios, err := p.GetScenarios()
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

	baseURL, _ := url.Parse(p.Config.QuayApi + "/" + p.Config.QuayRegistry + "/manifest/" + foundScenario.Digest)
	resp, _ := http.Get(baseURL.String())
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
	inputFieldsLabel := manifest.GetKrknctlLabel("krknctl.inputFields")

	if titleLabel == nil {
		return nil, fmt.Errorf("krkn.title LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
	}
	if descriptionLabel == nil {
		return nil, fmt.Errorf("krkn.description LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
	}
	if inputFieldsLabel == nil {
		return nil, fmt.Errorf("krkn.inputfields LABEL not found in tag: %s digest: %s", foundScenario.Name, foundScenario.Digest)
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
	re, err := regexp.Compile("^.*LABEL krknctl\\.title\\=\\\"(.*)\\\"")
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("title not found in image manifest")
	}
	return &matches[1], nil
}

func (p *ScenarioProvider) parseDescription(s string) (*string, error) {
	re, err := regexp.Compile("^.*LABEL krknctl\\.description\\=\\\"(.*)\\\"")
	if err != nil {
		return nil, err
	}
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("description not found in image manifest")
	}
	return &matches[1], nil
}

func (p *ScenarioProvider) parseInputFields(s string) ([]typing.InputField, error) {
	re, err := regexp.Compile(".*LABEL krknctl\\.inputFields\\='(.*)'$")
	if err != nil {
		return nil, err
	}
	var fields []typing.InputField
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil, errors.New("input fields not found in image manifest")
	}
	err = json.Unmarshal([]byte(matches[1]), &fields)
	if err != nil {
		return nil, err
	}
	return fields, nil
}
