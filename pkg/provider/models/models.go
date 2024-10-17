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
	Title          string `json:"title"`
	Description    string `json:"description"`
	KubeconfigPath string `json:"kubeconfig_path"`
	Fields         []typing.InputField
}

func (s *ScenarioDetail) GetFieldByName(name string) *typing.InputField {
	for _, v := range s.Fields {
		if *v.Name == name {
			return &v
		}
	}
	return nil
}
