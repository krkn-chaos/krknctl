package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/krkn-chaos/krknctl/pkg/container_manager/podman"
)

type ContainerManagerFactory struct {
}

func NewContainerManagerFactory() *ContainerManagerFactory {
	return &ContainerManagerFactory{}
}

func (f *ContainerManagerFactory) NewInstance(containerEnvironment container_manager.Environment, config *config.Config) container_manager.ContainerManager {
	switch containerEnvironment {
	case container_manager.Podman:
		return &podman.ContainerManager{
			Config: *config,
		}
	case container_manager.Docker:
		panic("docker container_manager not yet supported")
	}
	return nil
}
