package registryv2

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"io"
	"log"
	"net/http"
	"strings"
)

type ScenarioProvider struct {
	provider.BaseScenarioProvider
}

func (s *ScenarioProvider) GetRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	return s.getRegistryImages(registry)
}

func (s *ScenarioProvider) getRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	registryUri, err := registry.GetV2ScenarioRepositoryApiUri()
	if err != nil {
		return nil, err
	}

	body, err := s.queryRegistry(registryUri, registry.Username, registry.Password, registry.Token, "GET", registry.SkipTls)
	if err != nil {
		return nil, err
	}
	v2Tags := TagsV2{}
	var tags []models.ScenarioTag
	err = json.Unmarshal(*body, &v2Tags)
	if err != nil {
		return nil, err
	}
	for _, tag := range v2Tags.Tags {
		t := models.ScenarioTag{}
		t.Name = tag
		tags = append(tags, t)
	}

	return &tags, nil
}

func (s *ScenarioProvider) queryRegistry(uri string, username *string, password *string, token *string, method string, skipTls bool) (*[]byte, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTls,
		},
	}

	client := &http.Client{}
	client.Transport = tr
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	//req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	if token != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *token))
	}
	if username != nil {
		registryPassword := ""
		if password != nil {
			registryPassword = *password
		}
		req.SetBasicAuth(*username, registryPassword)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var deferErr error = nil
	defer func() {
		deferErr = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("image not found %s not found", uri)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("URI %s response: %d", uri, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &bodyBytes, deferErr
}

func (s *ScenarioProvider) GetGlobalEnvironment(registry *models.RegistryV2, scenario string) (*models.ScenarioDetail, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	scenarioTags, err := s.getRegistryImages(registry)
	if err != nil {
		return nil, err
	}
	var foundScenario *models.ScenarioTag = nil
	for _, tag := range *scenarioTags {
		if tag.Name == scenario {
			foundScenario = &tag
		}
	}
	if foundScenario == nil {
		return nil, fmt.Errorf("%s scenario not found in registry %s", scenario, registry.RegistryUrl)
	}
	baseImageRegistryUri, err := registry.GetV2ScenarioDetailApiUri(foundScenario.Name)
	if err != nil {
		return nil, err
	}

	scenarioDetail, err := s.getScenarioDetail(baseImageRegistryUri, foundScenario, true, registry)
	if err != nil {
		return nil, err
	}
	return scenarioDetail, nil
}

func (s *ScenarioProvider) GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	scenarios, err := s.GetRegistryImages(registry)
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

	registryUri, err := registry.GetV2ScenarioDetailApiUri(scenario)
	if err != nil {
		return nil, err
	}

	scenarioDetail, err := s.getScenarioDetail(registryUri, foundScenario, false, registry)
	if err != nil {
		return nil, err
	}
	return scenarioDetail, nil
}

func (s *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, random bool, seed *provider.ScaffoldSeed) (*string, error) {
	return provider.ScaffoldScenarios(scenarios, includeGlobalEnv, registry, s.Config, s, random, nil)
}

func (s *ScenarioProvider) getScenarioDetail(dataSource string, foundScenario *models.ScenarioTag, isGlobalEnvironment bool, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	body, err := s.queryRegistry(dataSource, registry.Username, registry.Password, registry.Token, "GET", registry.SkipTls)
	if err != nil {
		return nil, err
	}
	manifestV2 := ManifestV2{}
	if err = json.Unmarshal(*body, &manifestV2); err != nil {
		return nil, err
	}
	for _, l := range manifestV2.RawLayers {
		layer := LayerV1Compat{}
		if err = json.Unmarshal([]byte(l["v1Compatibility"]), &layer); err != nil {
			continue
		}
		manifestV2.Layers = append(manifestV2.Layers, layer)
	}
	scenarioDetail := models.ScenarioDetail{
		ScenarioTag: *foundScenario,
	}
	var titleLabel = ""
	var descriptionLabel = ""
	var inputFieldsLabel = ""
	if isGlobalEnvironment == true {
		titleLabel = s.Config.LabelTitleGlobal
		descriptionLabel = s.Config.LabelDescriptionGlobal
		inputFieldsLabel = s.Config.LabelInputFieldsGlobal

	} else {
		titleLabel = s.Config.LabelTitle
		descriptionLabel = s.Config.LabelDescription
		inputFieldsLabel = s.Config.LabelInputFields
	}
	var layers []provider.ContainerLayer
	for _, l := range manifestV2.Layers {
		layers = append(layers, l)
	}
	foundTitle := provider.GetKrknctlLabel(titleLabel, layers)
	foundDescription := provider.GetKrknctlLabel(descriptionLabel, layers)
	foundInputFields := provider.GetKrknctlLabel(inputFieldsLabel, layers)

	if foundTitle == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(titleLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}
	if foundDescription == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(descriptionLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}
	if foundInputFields == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(inputFieldsLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}

	parsedTitle, err := s.BaseScenarioProvider.ParseTitle(*foundTitle, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}
	parsedDescription, err := s.BaseScenarioProvider.ParseDescription(*foundDescription, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	parsedInputFields, err := s.BaseScenarioProvider.ParseInputFields(*foundInputFields, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	scenarioDetail.Title = *parsedTitle
	scenarioDetail.Description = *parsedDescription
	scenarioDetail.Fields = parsedInputFields
	return &scenarioDetail, nil

}
