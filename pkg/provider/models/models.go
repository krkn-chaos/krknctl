package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/registry"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"net/url"
	"os"
	"strconv"
	"time"
)

type RegistryV2 struct {
	Username            *string `form:"username"`
	Password            *string `form:"password"`
	Token               *string `form:"token"`
	RegistryUrl         string  `form:"registry_url"`
	ScenarioRepository  string  `form:"scenario_repository"`
	BaseImageRepository string  `form:"base_image_repository"`
	SkipTls             bool    `form:"skip_tls"`
}

func NewRegistryV2FromEnv(config config.Config) (*RegistryV2, error) {
	var registryV2 = RegistryV2{}
	registryUrl := os.Getenv(config.EnvPrivateRegistry)
	if registryUrl == "" {
		return nil, nil
	}
	registryV2.RegistryUrl = registryUrl

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

	skipTls := os.Getenv(config.EnvPrivateRegistrySkipTls)
	if skipTls == "" {
		registryV2.SkipTls = false
	} else {
		skip, err := strconv.ParseBool(skipTls)
		if err != nil {
			return nil, fmt.Errorf("wrong %s format can be `true` or `false`,  `%s` found instead", config.EnvPrivateRegistrySkipTls, skipTls)
		}
		registryV2.SkipTls = skip
	}
	fmt.Println("found private registry settings on ENV")
	return &registryV2, nil

}

func (r *RegistryV2) GetV2ScenarioRepositoryApiUri() (string, error) {
	prefix := "http://"
	if r.SkipTls == false {
		prefix = "https://"
	}
	registryUrl, err := url.Parse(fmt.Sprintf("%s/v2/%s/tags/list", prefix+r.RegistryUrl, r.ScenarioRepository))
	if err != nil {
		return "", err
	}
	return registryUrl.String(), nil
}

func (r *RegistryV2) GetV2BaseImageRepositoryApiUri() (string, error) {
	prefix := "http://"
	if r.SkipTls == false {
		prefix = "https://"
	}
	registryUrl, err := url.Parse(fmt.Sprintf("%s/v2/%s/tags/list", prefix+r.RegistryUrl, r.BaseImageRepository))
	if err != nil {
		return "", err
	}
	return registryUrl.String(), nil
}
func (r *RegistryV2) GetV2ScenarioDetailApiUri(scenario string) (string, error) {
	prefix := "http://"
	if r.SkipTls == false {
		prefix = "https://"
	}
	registryUrl, err := url.Parse(fmt.Sprintf("%s/v2/%s/manifests/%s", prefix+r.RegistryUrl, r.ScenarioRepository, scenario))
	if err != nil {
		return "", err
	}
	return registryUrl.String(), nil
}

func (r *RegistryV2) GetV2BaseImageScenarioDetailApiUri(tag string) (string, error) {
	prefix := "http://"
	if r.SkipTls == false {
		prefix = "https://"
	}
	registryUrl, err := url.Parse(fmt.Sprintf("%s/v2/%s/manifests/%s", prefix+r.RegistryUrl, r.BaseImageRepository, tag))
	if err != nil {
		return "", err
	}
	return registryUrl.String(), nil
}

func (r *RegistryV2) GetPrivateRegistryUri() string {
	return fmt.Sprintf("%s/%s", r.RegistryUrl, r.ScenarioRepository)
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

	encodedJSON, err := json.Marshal(authConfig)
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
