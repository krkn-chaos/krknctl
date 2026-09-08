// Package quay provides the implementation of the quay.io data provider
package quay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

type ScenarioProvider struct {
	provider.BaseScenarioProvider
}

func (p *ScenarioProvider) getRegistryImages(dataSource string, resolveSizes bool) (*[]models.ScenarioTag, error) {
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

		resp, err := http.Get(tagBaseURL.String())
		if err != nil {
			return nil, err
		}

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
		scenarioTag := models.ScenarioTag{
			Name:         tag.Name,
			LastModified: &tag.LastModified,
			Size:         tag.Size,
			Digest:       &tag.ManifestDigest,
		}
		scenarioTags = append(scenarioTags, scenarioTag)
	}
	if resolveSizes {
		p.populateMissingTagSizes(dataSource, scenarioTags)
	}

	return &scenarioTags, deferErr
}

func (p *ScenarioProvider) populateMissingTagSizes(dataSource string, tags []models.ScenarioTag) {
	const workerCount = 8
	jobs := make(chan int)
	var workers sync.WaitGroup
	count := workerCount
	if len(tags) < count {
		count = len(tags)
	}
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if tags[index].Size != nil && *tags[index].Size > 0 {
					continue
				}
				if size := p.getTagSize(dataSource, &tags[index]); size != nil {
					tags[index].Size = size
				}
			}
		}()
	}
	for index := range tags {
		if tags[index].Size == nil || *tags[index].Size == 0 {
			jobs <- index
		}
	}
	close(jobs)
	workers.Wait()
}

// getTagSize resolves a missing listing size from the image manifest. Quay's
// tag endpoint does not provide an aggregate size for manifest lists, so the
// selected platform descriptor is the authoritative value for multi-arch
// images. Metadata lookup failures leave the size unknown without hiding the
// tag from the listing.
func (p *ScenarioProvider) getTagSize(dataSource string, tag *models.ScenarioTag) *int64 {
	if tag.Digest == nil || *tag.Digest == "" {
		return nil
	}
	manifest, err := p.getResolvedManifest(dataSource, *tag.Digest)
	if err != nil {
		return nil
	}
	return manifestImageSize(manifest)
}

func (p *ScenarioProvider) getResolvedManifest(dataSource string, digest string) (Manifest, error) {
	body, err := p.getScenarioBytes(dataSource, digest)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, err
	}
	if !manifest.IsManifestList {
		return manifest, nil
	}
	if manifest.ManifestData == "" {
		return Manifest{}, errors.New("manifest list contains no manifest data")
	}
	var manifestList ManifestList
	if err := json.Unmarshal([]byte(manifest.ManifestData), &manifestList); err != nil {
		return Manifest{}, err
	}
	selected := manifestList.GetKrknctlManifest()
	if selected == nil {
		return Manifest{}, errors.New("manifest list contains no usable image manifest")
	}
	selectedBody, err := p.getScenarioBytes(dataSource, selected.Digest)
	if err != nil {
		return Manifest{}, err
	}
	if err := json.Unmarshal(selectedBody, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func manifestImageSize(manifest Manifest) *int64 {
	if manifest.LayerCompressedSize != "" {
		size, err := strconv.ParseInt(manifest.LayerCompressedSize, 10, 64)
		if err == nil && size > 0 {
			return &size
		}
	}
	var size int64
	for _, layer := range manifest.Layers {
		if layer.CompressedSize <= 0 {
			return nil
		}
		size += layer.CompressedSize
	}
	if size > 0 {
		return &size
	}
	return nil
}

func preserveKnownImageSize(existing *int64, manifest Manifest) *int64 {
	if imageSize := manifestImageSize(manifest); imageSize != nil {
		return imageSize
	}
	return existing
}

func (p *ScenarioProvider) GetRegistryImages(*models.RegistryV2) (*[]models.ScenarioTag, error) {
	dataSource, err := p.Config.GetQuayScenarioRepositoryAPIURI()
	if err != nil {
		return nil, err
	}

	scenarioTags, err := p.getRegistryImages(dataSource, true)
	if err != nil {
		return nil, err
	}
	return scenarioTags, nil
}

func (p *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, random bool, seed *provider.ScaffoldSeed) (*string, error) {
	return provider.ScaffoldScenarios(scenarios, includeGlobalEnv, registry, p.Config, p, random, seed)
}

// GetImageSignatureStatus reports the cosign signature state of a quay.io
// scenario image. The public quay registry needs no credentials, so the
// verification uses the default (zero) options; the registry argument is
// accepted only to satisfy the interface. It returns the unknown status with an
// error only if the image URI cannot be built from config.
func (p *ScenarioProvider) GetImageSignatureStatus(ctx context.Context, _ *models.RegistryV2, tag models.ScenarioTag) (verify.SignatureStatus, error) {
	imageURI, err := p.Config.GetQuayImageURI()
	if err != nil {
		return verify.SignatureUnknown, err
	}
	ref := provider.ImageReference(imageURI, tag)
	return p.BaseScenarioProvider.ImageSignatureStatus(ctx, ref, verify.Options{}), nil
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
		client := http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(baseURL.String())
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
	if foundScenario.Digest != nil {
		scenarioDigest = *foundScenario.Digest
	}
	manifest, err := p.getResolvedManifest(dataSource, scenarioDigest)
	if err != nil {
		return nil, err
	}
	foundScenario.Size = preserveKnownImageSize(foundScenario.Size, manifest)

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

	if err := p.BaseScenarioProvider.PopulateBooleanLabels(&scenarioDetail, layers, isGlobalEnvironment); err != nil {
		return nil, err
	}

	foundTitle := provider.GetKrknctlLabel(titleLabel, layers)
	foundDescription := provider.GetKrknctlLabel(descriptionLabel, layers)
	foundInputFields := provider.GetKrknctlLabel(inputFieldsLabel, layers)

	if foundTitle == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s: %w", strings.Replace(titleLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest, provider.ErrLabelNotFound)
	}
	if foundDescription == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s: %w", strings.Replace(descriptionLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest, provider.ErrLabelNotFound)
	}
	if foundInputFields == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s: %w", strings.Replace(inputFieldsLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest, provider.ErrLabelNotFound)
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
	scenarios, err := p.getRegistryImages(dataSource, false)
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
	scenarios, err := p.getRegistryImages(dataSource, false)
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
