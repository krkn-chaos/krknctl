package container_manager

import (
	"context"
	"fmt"
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

type ContainerManager interface {
	Run(
		image string,
		scenarioName string,
		containerRuntimeUri string,
		env map[string]string,
		cache bool,
		volumeMounts map[string]string,
		localKubeconfigPath string,
		kubeconfigMountPath string,

	) (*string, *context.Context, error)

	RunAttached(
		image string,
		scenarioName string,
		containerRuntimeUri string,
		env map[string]string,
		cache bool,
		volumeMounts map[string]string,
		localKubeconfigPath string,
		kubeconfigMountPath string,
	) (*string, error)

	CleanContainers() (*int, error)

	Attach(containerId *string, ctx *context.Context) error

	Kill(containerId *string, ctx *context.Context) error

	GetContainerRuntimeSocket(userId *int) (*string, error)
}
