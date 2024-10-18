package podman

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
	"log"
	"os"
	"time"
)

type ContainerManager struct{}

func (c *ContainerManager) Run(
	image string,
	scenarioName string,
	containerRuntimeUri string,
	env map[string]string,
	cache bool,
	volumeMounts map[string]string,
	localKubeconfigPath string,
	kubeconfigMountPath string,

) (*string, *context.Context, error) {
	conn, err := bindings.NewConnection(context.Background(), containerRuntimeUri)
	if err != nil {
		return nil, nil, err
	}
	//if the image exists but the digest has changed pulls the image again
	imageExists, err := images.Exists(conn, image, nil)
	if cache == false || imageExists == false {
		_, err = images.Pull(conn, image, nil)
	}

	if err != nil {
		return nil, nil, err
	}
	s := specgen.NewSpecGenerator(image, false)

	s.Name = fmt.Sprintf("krkn-%s-%d", scenarioName, time.Now().Unix())
	s.Env = env

	kubeconfigMount := specs.Mount{
		Destination: kubeconfigMountPath,
		Type:        string(mount.TypeBind),
		Source:      localKubeconfigPath,
		Options:     []string{"Z"},
		UIDMappings: nil,
		GIDMappings: nil,
	}
	s.Mounts = append(s.Mounts, kubeconfigMount)
	s.NetNS = specgen.Namespace{
		NSMode: "host",
	}
	createResponse, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		return nil, nil, err
	}
	return &createResponse.ID, &conn, nil
}

func (c *ContainerManager) RunAndStream(
	image string,
	scenarioName string,
	containerRuntimeUri string,
	env map[string]string,
	cache bool,
	volumeMounts map[string]string,
	localKubeconfigPath string,
	kubeconfigMountPath string,
) (*string, error) {

	containerId, conn, err := c.Run(image, scenarioName, containerRuntimeUri, env, cache, volumeMounts, localKubeconfigPath, kubeconfigMountPath)
	if err != nil {
		return nil, err
	}

	options := new(containers.AttachOptions).WithLogs(true).WithStream(true)

	err = containers.Attach(*conn, *containerId, os.Stdout, os.Stderr, nil, nil, options)
	if err != nil {
		return nil, err
	}

	if err != nil {
		log.Fatalf("Errore durante il waiting dell'attach: %v", err)
	}

	return containerId, nil

}

func (c *ContainerManager) GetContainerRuntimeUri() string {
	//TODO: return the uri by operatingsystem
	return "unix://run/user/4211263/podman/podman.sock"
}
