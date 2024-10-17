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
	"time"
)

type ContainerManager struct{}

func (c *ContainerManager) Run(image string,
	scenarioName string,
	containerRuntimeUri string,
	env map[string]string,
	cache bool,
	volumeMounts map[string]string,
	localKubeconfigPath string,
	kubeconfigMountPath string,

) (*string, error) {
	conn, err := bindings.NewConnection(context.Background(), containerRuntimeUri)
	if err != nil {
		return nil, err
	}
	//if the image exists but the digest has changed pulls the image again
	imageExists, err := images.Exists(conn, image, nil)
	if cache == false || imageExists == false {
		_, err = images.Pull(conn, image, nil)
	}

	if err != nil {
		return nil, err
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
		return nil, err
	}
	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		return nil, err
	}
	return &createResponse.ID, nil
}
