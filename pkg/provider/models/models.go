package models

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"net/url"
	"time"
)

type RegistryV2 struct {
	Username            *string `form:"username"`
	Password            *string `form:"password"`
	Token               *string `form:"token"`
	RegistryUrl         string  `form:"registry_url"`
	ScenarioRepository  string  `form:"scenario_repository"`
	BaseImageRepository string  `form:"base_image_repository"`
	UseTLS              bool    `form:"use_tls"`
}

func (r *RegistryV2) GetV2ScenarioRepositoryApiUri() (string, error) {
	prefix := "http://"
	if r.UseTLS {
		prefix = "https://"
	}
	url, err := url.Parse(fmt.Sprintf("%s/v2/%s/tags/list", prefix+r.RegistryUrl, r.ScenarioRepository))
	if err != nil {
		return "", err
	}
	return url.String(), nil
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
