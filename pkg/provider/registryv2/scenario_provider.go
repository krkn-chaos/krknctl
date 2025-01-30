package registryv2

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"io"
	"log"
	"net/http"
)

type ScenarioProvider struct {
}

func (s *ScenarioProvider) GetRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	//TODO implement me
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	registryUri, err := registry.GetV2ScenarioRepositoryApiUri()
	if err != nil {
		return nil, err
	}

	body, err := s.queryRegistry(registryUri, registry.Username, registry.Password, registry.Token, "GET")
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

func (s *ScenarioProvider) queryRegistry(uri string, username *string, password *string, token *string, method string) (*[]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	if token != nil {
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
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
		return nil, fmt.Errorf("URI %s not found, is the registry V2", uri)
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

func (s *ScenarioProvider) GetRegistryTags(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	panic("implement me")
}

func (s *ScenarioProvider) GetGlobalEnvironment(registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScenarioProvider) GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2) (*string, error) {
	//TODO implement me
	panic("implement me")
}
