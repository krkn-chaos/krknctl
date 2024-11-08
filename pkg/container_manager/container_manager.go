package container_manager

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"io"
)

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

func EnvironmentFromString(s string) ContainerRuntime {
	switch s {
	case "Podman":
		return Podman
	case "Docker":
		return Docker
	case "Both":
		return Both
	default:
		panic(fmt.Sprintf("unknown container environment: %q", s))
	}
}

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

type ScenarioSet map[string]ScenarioNode
type ResolvedGraph [][]string

type CommChannel struct {
	Layer           *int
	ScenarioId      *string
	ScenarioLogFile *string
	Err             error
}

type ContainerManager interface {
	Run(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string) (*string, *context.Context, error)

	RunAttached(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer) (*string, error)

	RunGraph(scenarios ScenarioSet,
		resolvedGraph ResolvedGraph,
		containerRuntimeUri string,
		extraEnv map[string]string,
		extraVolumeMounts map[string]string,
		cache bool,
		commChannel chan *CommChannel,
	)

	CleanContainers() (*int, error)

	Attach(containerId *string, ctx *context.Context, stdout io.Writer, stderr io.Writer) error

	Kill(containerId *string, ctx *context.Context) error

	GetContainerRuntimeSocket(userId *int) (*string, error)

	GetContainerRuntime() ContainerRuntime
}

func PrintDetectedContainerRuntime(containerManager *ContainerManager) {
	if containerManager == nil {
		panic("nil container manager")
	}
	green := color.New(color.FgGreen).SprintFunc()
	boldGreen := color.New(color.FgHiGreen, color.Bold).SprintFunc()
	fmt.Printf("%s %s\n\n", green("detected runtime:"), boldGreen((*containerManager).GetContainerRuntime().String()))
}
