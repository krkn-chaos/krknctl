package container_manager

import "context"

type Environment int64

const (
	Podman Environment = iota
	Docker
)

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

	RunAndStream(
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

	GetContainerRuntimeSocket(userId *int) (*string, error)
}
