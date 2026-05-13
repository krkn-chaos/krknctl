// Package quay provides the implementation of the quay.io data provider
package quay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

type ScenarioProvider struct {
	provider.BaseScenarioProvider
}

func (p *ScenarioProvider) getRegistryImages(dataSource string) (*[]models.ScenarioTag, error) {
	tagBaseURL, err := url.Parse(dataSource + "/tag")
	if err != nil {
		return nil, err
	}
	var deferErr error = nil
	cacheKey := tagBaseURL.String()
	bodyBytes := p.Cache.Get(cacheKey)
	if len(bodyBytes) == 0 {
		params := url.Values{}
		params.Add("onlyActiveTags", "true")
		params.Add("limit", "100")
		// currently paging support is not needed
		params.Add("page", "1")
		tagBaseURL.RawQuery = params.Encode()

		resp, _ := http.Get(tagBaseURL.String())

		defer func() {
			deferErr = resp.Body.Close()
		}()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New("failed to retrieve tags, " + tagBaseURL.String() + " returned: " + resp.Status)
		}

		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		p.Cache.Set(tagBaseURL.String(), bodyBytes)

	}
	var quayPage TagPage
	err = json.Unmarshal(bodyBytes, &quayPage)
	if err != nil {
		return nil, err
	}

	var scenarioTags []models.ScenarioTag
	for _, tag := range quayPage.Tags {
		scenarioTags = append(scenarioTags, models.ScenarioTag{
			Name:         tag.Name,
			LastModified: &tag.LastModified,
			Size:         &tag.Size,
			Digest:       &tag.ManifestDigest,
		})
	}

	return &scenarioTags, deferErr
}

func (p *ScenarioProvider) GetRegistryImages(*models.RegistryV2) (*[]models.ScenarioTag, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryAPIURI()
	if err != nil {
		return nil, err
	}

	scenarioTags, err := p.getRegistryImages(dataSource)
	if err != nil {
		return nil, err
	}
	return scenarioTags, nil
}

func (p *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, random bool, seed *provider.ScaffoldSeed) (*string, error) {
	return provider.ScaffoldScenarios(scenarios, includeGlobalEnv, registry, p.Config, p, random, seed)
}

func (p *ScenarioProvider) getScenarioBytes(dataSource string, scenarioDigest string) ([]byte,
	error) {
	var deferErr error = nil
	baseURL, err := url.Parse(dataSource + "/manifest/" + scenarioDigest)
	if err != nil {
		return nil, err
	}
	bodyBytes := p.Cache.Get(baseURL.String())
	if len(bodyBytes) == 0 {
		resp, err := http.Get(baseURL.String())
		if err != nil {
			return nil, err
		}

		defer func() {
			deferErr = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, errors.New("failed to retrieve scenario details, " + baseURL.String() + " returned: " + resp.Status)
		}
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		p.Cache.Set(baseURL.String(), bodyBytes)

	}
	return bodyBytes, deferErr
}

func (p *ScenarioProvider) getScenarioDetail(dataSource string, foundScenario *models.ScenarioTag, isGlobalEnvironment bool) (*models.ScenarioDetail, error) {

	scenarioDigest := ""
	if ((*foundScenario).Digest) != nil {
		scenarioDigest = *((*foundScenario).Digest)
	}
	bodyBytes, err := p.getScenarioBytes(dataSource, scenarioDigest)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	err = json.Unmarshal(bodyBytes, &manifest)
	if err != nil {
		return nil, err
	}

	// if the manifest is a manifestList (multiarch image) image metadata
	// will be fetched from the first available image in the registry
	// keeps retrocompatibility with registries with no manifests

	if manifest.IsManifestList {
		if manifest.ManifestData == "" {
			return nil, errors.New("scenario image is a manifest without data, " +
				"impossible to fetch details")
		}
		var ml ManifestList
		err = json.Unmarshal([]byte(manifest.ManifestData), &ml)
		if err != nil {
			return nil, err
		}
		imageHash := ml.GetFirstAvailableHash()
		if imageHash == nil {
			return nil, errors.New("scenario image not found for target architecture")
		}

		bodyBytes, err = p.getScenarioBytes(dataSource, *imageHash)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(bodyBytes, &manifest)
		if err != nil {
			return nil, err
		}
	}

	scenarioDetail := models.ScenarioDetail{
		ScenarioTag: *foundScenario,
	}
	var titleLabel = ""
	var descriptionLabel = ""
	var inputFieldsLabel = ""
	if isGlobalEnvironment {
		titleLabel = p.Config.LabelTitleGlobal
		descriptionLabel = p.Config.LabelDescriptionGlobal
		inputFieldsLabel = p.Config.LabelInputFieldsGlobal

	} else {
		titleLabel = p.Config.LabelTitle
		descriptionLabel = p.Config.LabelDescription
		inputFieldsLabel = p.Config.LabelInputFields
	}
	var layers []provider.ContainerLayer
	for _, l := range manifest.Layers {
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

	parsedTitle, err := p.BaseScenarioProvider.ParseTitle(*foundTitle, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}
	parsedDescription, err := p.ParseDescription(*foundDescription, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	parsedInputFields, err := p.ParseInputFields(*foundInputFields, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	scenarioDetail.Title = *parsedTitle
	scenarioDetail.Description = *parsedDescription
	scenarioDetail.Fields = parsedInputFields
	return &scenarioDetail, nil
}

func (p *ScenarioProvider) GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryAPIURI()
	if err != nil {
		return nil, err
	}
	scenarios, err := p.GetRegistryImages(registry)
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

func (p *ScenarioProvider) GetGlobalEnvironment(registry *models.RegistryV2, scenario string) (*models.ScenarioDetail, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryAPIURI()
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
		if scenarioTag.Name == scenario {
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
