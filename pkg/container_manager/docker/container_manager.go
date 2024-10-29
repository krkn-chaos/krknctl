package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/briandowns/spinner"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	images "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/krkn-chaos/krknctl/internal/config"
	"io"
	"log"
	"os"
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
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(containerRuntimeUri))
	if err != nil {
		return nil, nil, err
	}
	exists, err := imageExists(ctx, cli, image, "")
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
	volumes = append(volumes, mount.Mount{
		Type:   mount.TypeBind,
		Source: localKubeconfigPath,
		Target: kubeconfigMountPath,
	})
	for k, v := range volumeMounts {
		volumes = append(volumes, mount.Mount{
			Type:   mount.TypeBind,
			Source: v,
			Target: k,
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

	containerId, ctx, err := c.Run(image, scenarioName, containerRuntimeUri, env, cache, volumeMounts, localKubeconfigPath, kubeconfigMountPath)
	if err != nil {
		return nil, err
	}
	cli, err := dockerClientFromContext(*ctx)
	if err != nil {
		return nil, err
	}

	options := dockercontainer.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Follow:     true,
	}

	reader, err := cli.ContainerLogs(context.Background(), *containerId, options)
	if err != nil {
		log.Fatalf("Errore ottenendo i log: %v", err)
	}
	defer reader.Close()

	// Crea un writer per stampare i log in tempo reale
	go func() {

		_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
		if err != nil {
			log.Fatalf("Errore copiando i log: %v", err)
		}
	}()

	statusCh, errCh := cli.ContainerWait(*ctx, *containerId, dockercontainer.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case <-statusCh:
		return containerId, nil
	}
	return nil, errors.New("scenario exited for an unknown error")
}

func (c *ContainerManager) CleanContainers() (*int, error) {
	return nil, nil
}

func (c *ContainerManager) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return &c.Config.DockerSocketRoot, nil
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

func imageExists(ctx context.Context, cli *client.Client, imageName, expectedDigest string) (bool, error) {

	images, err := cli.ImageList(ctx, images.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, img := range images {
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

	var totalSize int
	s := spinner.New(spinner.CharSets[39], 100*time.Millisecond)
	s.Suffix = "pulling image...."
	s.Start()

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
	}
	s.Stop()

	return nil
}
