package providers

import (
	"encoding/json"
	"errors"
	"github.com/krkn-chaos/krknctl/internal/config"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type QuayTagPage struct {
	Tags          []QuayTag `json:"tags"`
	Page          int       `json:"page"`
	HasAdditional bool      `json:"has_additional"`
}

type QuayTag struct {
	Name           string    `json:"name"`
	Reversion      bool      `json:"reversion"`
	StartTimeStamp int64     `json:"start_ts"`
	ManifestDigest string    `json:"manifest_digest"`
	IsManifestList bool      `json:"is_manifest_list"`
	Size           int64     `json:"size"`
	LastModified   time.Time `json:"last_modified"`
}

type alias QuayTag

func (q *QuayTag) UnmarshalJSON(bytes []byte) error {
	aux := &struct {
		LastModified string `json:"last_modified"`
		*alias
	}{
		alias: (*alias)(q),
	}
	if err := json.Unmarshal(bytes, &aux); err != nil {
		return err
	}

	layout := "Mon, 02 Jan 2006 15:04:05 -0700"
	parsedTime, err := time.Parse(layout, aux.LastModified)
	if err != nil {
		return err
	}

	q.LastModified = parsedTime

	return nil
}

type QuayScenarioProvider struct {
	config *config.Config
}

func (p *QuayScenarioProvider) GetScenarios() (*[]ScenarioTag, error) {
	baseURL, _ := url.Parse(p.config.QuayApi + "/" + p.config.QuayRegistry + "/tag")
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
	var quayPage QuayTagPage
	err = json.Unmarshal(bodyBytes, &quayPage)

	var scenarioTags []ScenarioTag
	for _, tag := range quayPage.Tags {
		scenarioTags = append(scenarioTags, ScenarioTag{
			Name:         tag.Name,
			LastModified: tag.LastModified,
			Size:         tag.Size,
			Digest:       tag.ManifestDigest,
		})
	}

	return &scenarioTags, nil
}

func (p *QuayScenarioProvider) GetScenarioDetail(scenario string) (*ScenarioDetail, error) {
	scenarios, err := p.GetScenarios()
	if err != nil {
		return nil, err
	}
	var foundScenario *ScenarioTag = nil
	for _, scenarioTag := range *scenarios {
		if scenarioTag.Name == scenario {
			foundScenario = &scenarioTag
		}
	}
	if foundScenario == nil {
		return nil, nil
	}

	baseURL, _ := url.Parse(p.config.QuayApi + "/" + p.config.QuayRegistry + "/tag")
	params := url.Values{}
	params.Add("onlyActiveTags", "true")
	params.Add("limit", "100")
	// currently paging support is not needed
	params.Add("page", "1")
	baseURL.RawQuery = params.Encode()

	resp, _ := http.Get(baseURL.String())

}
