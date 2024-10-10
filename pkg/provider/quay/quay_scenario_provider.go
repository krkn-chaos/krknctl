package quay

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"

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

	title := manifest.GetKrknctlLabel("krknctl.title")
	description := manifest.GetKrknctlLabel("krknctl.description")
	inputFields := manifest.GetKrknctlLabel("krknctl.inputFields")

	fmt.Println(title)
	fmt.Println(description)
	fmt.Println(inputFields)

	fmt.Println(string(bodyBytes))
	return nil, nil

}
