package podman

import (
	"context"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types/mount"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/opencontainers/runtime-spec/specs-go"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

type ContainerManager struct {
	Config config.Config
}

func (c *ContainerManager) Run(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string) (*string, *context.Context, error) {
	conn, err := bindings.NewConnection(context.Background(), containerRuntimeUri)
	if err != nil {
		return nil, nil, err
	}
	//if the image exists but the digest has changed pulls the image again
	imageExists, err := images.Exists(conn, image, nil)
	if cache == false || imageExists == false {
		s := spinner.New(spinner.CharSets[39], 100*time.Millisecond)
		s.Suffix = fmt.Sprintf("pulling image %s....", image)
		s.Start()
		options := images.PullOptions{}
		options.WithQuiet(true)
		_, err = images.Pull(conn, image, &options)
		if err != nil {
			return nil, nil, err
		}
		s.Stop()
	}

	if err != nil {
		return nil, nil, err
	}
	s := specgen.NewSpecGenerator(image, false)

	s.Name = fmt.Sprintf("%s-%s-%d", c.Config.ContainerPrefix, scenarioName, time.Now().Unix())
	s.Env = env
	for k, v := range volumeMounts {
		containerMount := specs.Mount{
			Destination: v,
			Type:        string(mount.TypeBind),
			Source:      k,
			Options:     []string{"Z"},
			UIDMappings: nil,
			GIDMappings: nil,
		}
		s.Mounts = append(s.Mounts, containerMount)
	}

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

func (c *ContainerManager) RunAttached(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string) (*string, error) {
	_, err := color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to terminate the scenario")
	if err != nil {
		return nil, err
	}
	// to make the above message readable
	time.Sleep(2)
	containerId, conn, err := c.Run(image, scenarioName, containerRuntimeUri, env, cache, volumeMounts)
	if err != nil {
		return nil, err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	kill, err := c.attach(containerId, conn, sigCh)

	if err != nil {
		return nil, err
	}
	if kill {
		err := c.Kill(containerId, conn)
		if err != nil {
			return nil, err
		}
		_, err = color.New(color.FgRed, color.Underline).Println(fmt.Sprintf("container %s killed", *containerId))
		if err != nil {
			return nil, err
		}
	}
	return containerId, nil

}

func (c *ContainerManager) attach(containerId *string, conn *context.Context, signalChannel chan os.Signal) (bool, error) {

	options := new(containers.AttachOptions).WithLogs(true).WithStream(true).WithDetachKeys("ctrl-c")

	errorChannel := make(chan error, 1)
	finishChannel := make(chan bool, 1)
	go func() {
		err := containers.Attach(*conn, *containerId, nil, os.Stdout, os.Stderr, nil, options)
		if err != nil {
			errorChannel <- err
		}
		finishChannel <- true
	}()

	select {
	case <-finishChannel:
		return false, nil
	case <-signalChannel:
		return true, nil
	case err := <-errorChannel:
		return false, err
	}
}

func (c *ContainerManager) Attach(containerId *string, conn *context.Context) error {
	_, err := color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to stop streaming scenario output (scenario won't be interrupted)")
	if err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	interrupted, err := c.attach(containerId, conn, sigCh)
	if err != nil {
		return err
	}
	if interrupted {
		_, err = color.New(color.FgRed, color.Underline).Println(fmt.Sprintf("scenario output terminated, container %s still running", *containerId))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainerManager) CleanContainers() (*int, error) {
	_true := true
	nameRegex, err := regexp.Compile(fmt.Sprintf("^%s.*-[0-9]+$", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}
	socket, err := c.GetContainerRuntimeSocket(nil)
	if err != nil {
		return nil, err
	}
	conn, err := bindings.NewConnection(context.Background(), *socket)
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

func (c *ContainerManager) Kill(containerId *string, ctx *context.Context) error {
	err := containers.Kill(*ctx, *containerId, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerManager) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return container_manager.GetSocketByContainerEnvironment(container_manager.Podman, c.Config, userId)
}
