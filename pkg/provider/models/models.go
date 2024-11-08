package models

import (
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"time"
)

type ScenarioTag struct {
	Name         string    `json:"name"`
	Digest       string    `json:"digest"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

type ScenarioDetail struct {
	ScenarioTag
	Title       string `json:"title"`
	Description string `json:"description"`
	Fields      []typing.InputField
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
