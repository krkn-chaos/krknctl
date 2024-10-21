package podman

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types/mount"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/opencontainers/runtime-spec/specs-go"
	"log"
	"os"
	"os/user"
	"regexp"
	"time"
)

type ContainerManager struct {
	Config config.Config
}

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

	s.Name = fmt.Sprintf("%s-%s-%d", c.Config.ContainerPrefix, scenarioName, time.Now().Unix())
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

	err = containers.Attach(*conn, *containerId, nil, os.Stdout, os.Stderr, nil, options)
	if err != nil {
		return nil, err
	}

	if err != nil {
		log.Fatalf("Errore durante il waiting dell'attach: %v", err)
	}

	return containerId, nil

}

func (c *ContainerManager) CleanContainers() (*int, error) {
	_true := true
	nameRegex, err := regexp.Compile(fmt.Sprintf("^%s.*-[0-9]+$", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}
	conn, err := bindings.NewConnection(context.Background(), c.GetContainerRuntimeUri())
	if err != nil {
		return nil, err
	}

	foundContainers, err := containers.List(conn, &containers.ListOptions{
		All: &_true,
	})
	if err != nil {
		return nil, err
	}
	deletedContainers := 0

	for _, c := range foundContainers {
		for _, n := range c.Names {
			if nameRegex.MatchString(n) {
				_, err := containers.Remove(conn, n, &containers.RemoveOptions{
					Force: &_true,
				})
				if err != nil {
					return nil, err
				}
				deletedContainers++
			}
		}
	}

	return &deletedContainers, nil
}

func (c *ContainerManager) GetContainerRuntimeUri() string {

	currentUser, _ := user.Current()
	if currentUser.Uid == "0" {
		return "/run/podman/podman.sock"
	}
	return fmt.Sprintf("unix://run/user/%s/podman/podman.sock", currentUser.Uid)

}
