// Package models provides the data models for the container runtime environment
package models

import "github.com/krkn-chaos/krknctl/pkg/provider/models"

type ContainerRuntime int64

func (e ContainerRuntime) String() string {
	switch e {
	case Podman:
		return "Podman"
	case Docker:
		return "Docker"
	case Both:
		return "Both"

	}
	return "Unknown"
}

const (
	Podman ContainerRuntime = iota
	Docker
	Both
)

type ScenarioNode struct {
	Scenario
	Parent *string `json:"depends_on,omitempty"`
}

func (s ScenarioNode) GetParent() *string {
	return s.Parent
}

type Scenario struct {
	// the only purpose of this attribute is to put a comment in the json
	Comment string            `json:"_comment,omitempty"`
	Image   string            `json:"image,omitempty"`
	Name    string            `json:"name,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Volumes map[string]string `json:"volumes,omitempty"`
}

type ScenarioContainer struct {
	Scenario       *Scenario
	ScenarioDetail *models.ScenarioDetail
	Container      *Container
}

type Container struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Image      string `json:"image,omitempty"`
	Started    int64  `json:"started,omitempty"`
	Status     string `json:"status,omitempty"`
	ExitStatus int    `json:"exit_status"`
}

type ScenarioSet map[string]ScenarioNode
type ResolvedGraph [][]string

type GraphCommChannel struct {
	Layer           *int
	ScenarioID      *string
	ScenarioLogFile *string
	Err             error
}
