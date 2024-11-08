package docker

import (
	"context"
	"errors"
	"fmt"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	images "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ContainerManager struct {
	Config           config.Config
	ContainerRuntime container_manager.ContainerRuntime
}

func (c *ContainerManager) Run(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string) (*string, *context.Context, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(containerRuntimeUri))
	if err != nil {
		return nil, nil, err
	}
	exists, err := ImageExists(ctx, cli, image, "")
	if err != nil {
		return nil, nil, err
	}
	if cache == false || exists == false {
		err := pullImage(ctx, cli, image)
		if err != nil {
			return nil, nil, err
		}
	}

	var envVars []string
	for k, v := range env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	var volumes []mount.Mount
	for k, v := range volumeMounts {
		volumes = append(volumes, mount.Mount{
			Type:   mount.TypeBind,
			Source: k,
			Target: v,
		})
	}
	resp, err := cli.ContainerCreate(ctx, &dockercontainer.Config{
		Image: image,
		Env:   envVars,
	}, &dockercontainer.HostConfig{
		Mounts:      volumes,
		NetworkMode: "host",
	}, nil, nil, fmt.Sprintf("%s-%s-%d", c.Config.ContainerPrefix, scenarioName, time.Now().Unix()))
	if err != nil {
		return nil, nil, err
	}

	if err := cli.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return nil, nil, err
	}

	ctxWithClient := contextWithDockerClient(ctx, cli)

	return &resp.ID, &ctxWithClient, nil
}

func (c *ContainerManager) RunAttached(image string, scenarioName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer) (*string, error) {
	_, err := color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to terminate the scenario")
	if err != nil {
		return nil, err
	}
	// to make the above message readable
	time.Sleep(2)
	containerId, ctx, err := c.Run(image, scenarioName, containerRuntimeUri, env, cache, volumeMounts)
	if err != nil {
		return nil, err
	}

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	kill, err := c.attach(containerId, ctx, signalChan, stdout, stderr)

	if err != nil {
		return nil, err
	}

	if kill {
		err = c.Kill(containerId, ctx)
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

func (c *ContainerManager) RunSerialPlan() {
	//TODO implement me
	panic("implement me")
}

func (c *ContainerManager) attach(containerId *string, ctx *context.Context, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer) (bool, error) {
	cli, err := dockerClientFromContext(*ctx)
	if err != nil {
		return false, err
	}
	options := dockercontainer.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Follow:     true,
	}

	reader, err := cli.ContainerLogs(context.Background(), *containerId, options)
	if err != nil {
		return false, err
	}
	defer reader.Close()
	errorChan := make(chan error)
	finishChan := make(chan error)

	// copies demultiplexed reader to Stdout and Stderr
	go func() {
		_, err := stdcopy.StdCopy(stdout, stderr, reader)
		if err != nil {
			errorChan <- err
		}
	}()

	// waits for:
	// - an error coming from stdcopy.Stdcopy
	// - the container to finish run
	// and signals to the outer select
	go func() {
		respCh, errCH := cli.ContainerWait(*ctx, *containerId, dockercontainer.WaitConditionNotRunning)
		select {
		case err := <-errCH:
			errorChan <- err
		case <-respCh:
			finishChan <- nil
		}
	}()

	// waits for:
	// - the previous function to either return an error or return a finish
	// - a SIGTERM in that case returns true so the container is killed (for RunAttached)
	select {
	case err := <-errorChan:
		return false, err
	case <-signalChannel:
		return true, nil
	case err := <-finishChan:
		if err != nil {
			return true, err
		}
		return false, nil
	}

}

func (c *ContainerManager) Attach(containerId *string, ctx *context.Context, stdout io.Writer, stderr io.Writer) error {
	_, err := color.New(color.FgGreen, color.Underline).Println("hit CTRL+C to stop streaming scenario output (scenario won't be interrupted)")
	if err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	interrupted, err := c.attach(containerId, ctx, sigCh, stdout, stderr)
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

func (c *ContainerManager) Kill(containerId *string, ctx *context.Context) error {
	cli, err := dockerClientFromContext(*ctx)
	if err != nil {
		return err
	}

	err = cli.ContainerKill(*ctx, *containerId, "KILL")
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerManager) CleanContainers() (*int, error) {
	return nil, nil
}

func (c *ContainerManager) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return container_manager.GetSocketByContainerEnvironment(container_manager.Docker, c.Config, userId)
}

func (c *ContainerManager) checkImageAndPull(cli *client.Client, ctx context.Context, container_image string, digest string, cache bool) (*bool, error) {
	_true := true
	images, err := cli.ImageList(ctx, dockerimage.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		if cache == false || i.ID != digest {
			_, err := cli.ImagePull(ctx, container_image, dockerimage.PullOptions{})
			if err != nil {
				return nil, err
			}
		}
	}
	return &_true, nil

}

func (c *ContainerManager) RunGraph(scenarios container_manager.ScenarioSet, resolvedGraph container_manager.ResolvedGraph, containerRuntimeUri string, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *container_manager.CommChannel) {
	//TODO implement me
	panic("implement me")
}

type contextKey string

const clientKey contextKey = "dockerClient"

func contextWithDockerClient(ctx context.Context, cli *client.Client) context.Context {
	return context.WithValue(ctx, clientKey, cli)
}

func dockerClientFromContext(ctx context.Context) (*client.Client, error) {
	cli, ok := ctx.Value(clientKey).(*client.Client)
	if !ok {
		return nil, errors.New("dockerClient not found in context")
	}
	return cli, nil
}

func ImageExists(ctx context.Context, cli *client.Client, imageName, expectedDigest string) (bool, error) {

	imgs, err := cli.ImageList(ctx, images.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, img := range imgs {
		if img.RepoTags != nil && len(img.RepoTags) > 0 && img.RepoTags[0] == imageName {
			return true, nil
		}
	}
	return false, nil
}

func pullImage(ctx context.Context, cli *client.Client, imageName string) error {
	reader, err := cli.ImagePull(ctx, imageName, images.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	/*
		********* update to send updates to a channel ********
		var totalSize int

		decoder := json.NewDecoder(reader)

			for {

				var message jsonmessage.JSONMessage
				if err := decoder.Decode(&message); err == io.EOF {
					break
				} else if err != nil {
					return err
				}

				if message.Progress != nil && message.Progress.Total > 0 {
					if totalSize == 0 {
						totalSize = int(message.Progress.Total)
					}
					if message.Progress.Current > 0 {
						s.Suffix = fmt.Sprintf("Downloading image %s: %d/%d MB ", imageName, int(message.Progress.Current/1024/1024), totalSize/1024/1024)
					}
				}
			}*/

	return nil
}

func (c *ContainerManager) GetContainerRuntime() container_manager.ContainerRuntime {
	return c.ContainerRuntime
}
