// Package models provides the data models used by the data providers to exchange data
package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/registry"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/typing"
)

type RegistryV2 struct {
	Username           *string `form:"username"`
	Password           *string `form:"password"` //#nosec G117
	Token              *string `form:"token"`
	RegistryURL        string  `form:"registry_url"`
	ScenarioRepository string  `form:"scenario_repository"`
	SkipTLS            bool    `form:"skip_tls"`
	Insecure           bool    `form:"insecure"`
}

func NewRegistryV2FromEnv(config config.Config) (*RegistryV2, error) {
	var registryV2 = RegistryV2{}
	registryURL := os.Getenv(config.EnvPrivateRegistry)
	if registryURL == "" {
		return nil, nil
	}
	registryV2.RegistryURL = registryURL

	scenarioRepository := os.Getenv(config.EnvPrivateRegistryScenarios)
	if scenarioRepository == "" {
		return nil, fmt.Errorf("environment variable %s not set", config.EnvPrivateRegistryScenarios)
	}
	registryV2.ScenarioRepository = scenarioRepository

	username := os.Getenv(config.EnvPrivateRegistryUsername)
	password := os.Getenv(config.EnvPrivateRegistryPassword)
	token := os.Getenv(config.EnvPrivateRegistryToken)

	registryV2.Username = &username
	registryV2.Password = &password
	registryV2.Token = &token

	skipTLS := os.Getenv(config.EnvPrivateRegistrySkipTLS)
	if skipTLS == "" {
		registryV2.SkipTLS = false
	} else {
		skip, err := strconv.ParseBool(skipTLS)
		if err != nil {
			return nil, fmt.Errorf("wrong %s format can be `true` or `false`,  `%s` found instead", config.EnvPrivateRegistrySkipTLS, skipTLS)
		}
		registryV2.SkipTLS = skip
	}

	insecure := os.Getenv(config.EnvPrivateRegistryInsecure)
	if insecure == "" {
		registryV2.Insecure = false
	} else {
		isInsecure, err := strconv.ParseBool(insecure)
		if err != nil {
			return nil, fmt.Errorf("wrong %s format can be `true` or `false`,  `%s` found instead", config.EnvPrivateRegistryInsecure, insecure)
		}
		registryV2.Insecure = isInsecure
	}

	return &registryV2, nil

}

func (r *RegistryV2) GetV2ScenarioRepositoryAPIURI() (string, error) {
	prefix := "http://"
	if !r.Insecure {
		prefix = "https://"
	}
	registryURL, err := url.Parse(fmt.Sprintf("%s/v2/%s/tags/list", prefix+r.RegistryURL, r.ScenarioRepository))
	if err != nil {
		return "", err
	}
	return registryURL.String(), nil
}

func (r *RegistryV2) GetV2ScenarioDetailAPIURI(scenario string) (string, error) {
	prefix := "http://"
	if !r.Insecure {
		prefix = "https://"
	}
	registryURL, err := url.Parse(fmt.Sprintf("%s/v2/%s/manifests/%s", prefix+r.RegistryURL, r.ScenarioRepository, scenario))
	if err != nil {
		return "", err
	}
	return registryURL.String(), nil
}

func (r *RegistryV2) GetPrivateRegistryURI() string {
	return fmt.Sprintf("%s/%s", r.RegistryURL, r.ScenarioRepository)
}

func (r *RegistryV2) ToDockerV2AuthString() (*string, error) {
	authConfig := registry.AuthConfig{}
	if r.Token != nil {
		authConfig.RegistryToken = *r.Token
	} else {
		if r.Username != nil {
			authConfig.Username = *r.Username
			authConfig.Password = *r.Password
		} else {
			return nil, nil
		}
	}

	encodedJSON, err := json.Marshal(authConfig) //#nosec G117
	if err != nil {
		return nil, err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return &authStr, nil

}

type ScenarioTag struct {
	Name         string     `json:"name"`
	Digest       *string    `json:"digest"`
	Size         *int64     `json:"size"`
	LastModified *time.Time `json:"last_modified"`
}

type ScenarioDetail struct {
	ScenarioTag
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Fields      []typing.InputField `json:"fields"`
}

func (s *ScenarioDetail) GetFieldByName(name string) *typing.InputField {
	for _, v := range s.Fields {
		if *v.Name == name {
			return &v
		}
	}
	return nil
}

func (s *ScenarioDetail) GetFieldByEnvVar(envVar string) *typing.InputField {
	for _, v := range s.Fields {
		if *v.Variable == envVar {
			return &v
		}
	}
	return nil
}

func (s *ScenarioDetail) GetFileFieldByMountPath(mountPath string) *typing.InputField {
	for _, v := range s.Fields {
		if v.Type == typing.File && v.MountPath != nil && *v.MountPath == mountPath {
			return &v
		}
	}
	return nil
}
