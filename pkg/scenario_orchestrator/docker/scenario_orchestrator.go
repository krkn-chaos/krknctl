package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	images "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/krkn-chaos/krknctl/internal/config"
	provider_models "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	orchestrator_models "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type ScenarioOrchestrator struct {
	Config           config.Config
	ContainerRuntime orchestrator_models.ContainerRuntime
}

func (c *ScenarioOrchestrator) Run(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, commChan *chan *string) (*string, *context.Context, error) {

	ctx, err := c.Connect(containerRuntimeUri)
	if err != nil {
		return nil, nil, err
	}

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	exists, err := ImageExists(ctx, cli, image, "")
	if err != nil {
		return nil, nil, err
	}
	if cache == false || exists == false {
		err := pullImage(ctx, cli, image, commChan)
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
	}, nil, nil, containerName)
	if err != nil {
		return nil, nil, err
	}

	if err := cli.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return nil, nil, err
	}

	ctxWithClient := contextWithDockerClient(ctx, cli)

	return &resp.ID, &ctxWithClient, nil
}

func (c *ScenarioOrchestrator) Attach(containerId *string, ctx *context.Context, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer) (bool, error) {
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

func (c *ScenarioOrchestrator) Kill(containerId *string, ctx *context.Context) error {
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

func (c *ScenarioOrchestrator) CleanContainers() (*int, error) {
	panic("not yet implemented on docker")
}
func (c *ScenarioOrchestrator) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return utils.GetSocketByContainerEnvironment(orchestrator_models.Docker, c.Config, userId)
}

func (c *ScenarioOrchestrator) checkImageAndPull(cli *client.Client, ctx context.Context, containerImage string, digest string, cache bool) (*bool, error) {
	_true := true
	containerImages, err := cli.ImageList(ctx, dockerimage.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, i := range containerImages {
		if cache == false || i.ID != digest {
			_, err := cli.ImagePull(ctx, containerImage, dockerimage.PullOptions{})
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

func pullImage(ctx context.Context, cli *client.Client, imageName string, commChan *chan *string) error {
	reader, err := cli.ImagePull(ctx, imageName, images.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

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
				if commChan != nil {
					message := fmt.Sprintf("Downloading image %s  %s ", imageName, message.Progress.String())
					*commChan <- &message
				}

			}
		}
	}

	if commChan != nil {
		close(*commChan)
	}

	return nil
}

func (c *ScenarioOrchestrator) GetContainerRuntime() orchestrator_models.ContainerRuntime {
	return c.ContainerRuntime
}

func (c *ScenarioOrchestrator) ListRunningContainers(containerRuntimeUri string) (*map[int64]orchestrator_models.Container, error) {
	scenarios := make(map[int64]orchestrator_models.Container)
	scenarioNameRegex, err := regexp.Compile(fmt.Sprintf("%s-.*-([0-9]+)", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}
	ctx, err := c.Connect(containerRuntimeUri)
	if err != nil {
		return nil, err
	}

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Recuperare i container attualmente in esecuzione
	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		if container.State == c.Config.DockerRunningState {
			groups := scenarioNameRegex.FindStringSubmatch(container.Names[0])
			if groups != nil && len(groups) > 1 {
				index, err := strconv.ParseInt(groups[1], 10, 64)
				if err != nil {
					return nil, err
				}
				scenarios[index] = orchestrator_models.Container{
					Name:    container.Names[0],
					Id:      container.ID,
					Image:   container.Image,
					Started: index,
				}
			}
		}
	}
	return &scenarios, nil

}

func (c *ScenarioOrchestrator) InspectRunningScenario(container orchestrator_models.Container, containerRuntimeUri string) (*orchestrator_models.RunningScenario, error) {
	runningScenario := &orchestrator_models.RunningScenario{}
	scenario := orchestrator_models.Scenario{}
	scenario.Volumes = make(map[string]string)
	scenario.Env = make(map[string]string)
	runningScenario.Container = &container
	ctx, err := c.Connect(containerRuntimeUri)
	if err != nil {
		return nil, err
	}

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inspectData, err := cli.ContainerInspect(ctx, container.Id)
	if err != nil {
		return nil, err
	}
	scenarioDetail := provider_models.ScenarioDetail{}
	scenarioDetail.Digest = inspectData.ContainerJSONBase.Image
	imageAndTag := strings.Split(inspectData.Config.Image, ":")
	if len(imageAndTag) == 2 {
		scenarioDetail.Name = imageAndTag[1]
	} else {
		scenarioDetail.Name = imageAndTag[0]
	}

	for k, v := range inspectData.Config.Labels {
		if k == c.Config.LabelTitle {
			scenarioDetail.Title = v
		}
		if k == c.Config.LabelDescription {
			scenarioDetail.Description = v
		}
		if k == c.Config.LabelInputFields {
			var inputFields []typing.InputField
			err := json.Unmarshal([]byte(v), &inputFields)
			if err != nil {
				return nil, err
			}
			scenarioDetail.Fields = inputFields
		}
	}
	runningScenario.ScenarioDetail = &scenarioDetail

	scenario.Name = scenarioDetail.Name
	scenario.Image = inspectData.Config.Image
	for _, v := range inspectData.Mounts {
		scenario.Volumes[v.Source] = v.Destination
	}

	for _, v := range inspectData.Config.Env {
		kv := strings.Split(v, "=")
		if len(kv) == 2 {
			scenario.Env[kv[0]] = kv[1]
		}
	}

	runningScenario.Scenario = &scenario
	return runningScenario, nil
}

func (c *ScenarioOrchestrator) Connect(containerRuntimeUri string) (context.Context, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(containerRuntimeUri))
	if err != nil {
		return nil, err
	}

	ctxWithClient := contextWithDockerClient(ctx, cli)
	return ctxWithClient, nil
}

// common functions

func (c *ScenarioOrchestrator) AttachWait(containerId *string, ctx *context.Context, stdout io.Writer, stderr io.Writer) (*bool, error) {

	interrupted, err := scenario_orchestrator.CommonAttachWait(containerId, ctx, stdout, stderr, c)
	if err != nil {
		return nil, err
	}

	return &interrupted, nil
}

func (c *ScenarioOrchestrator) RunAttached(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, commChan *chan *string) (*string, error) {
	containerId, err := scenario_orchestrator.CommonRunAttached(image, containerName, containerRuntimeUri, env, cache, volumeMounts, stdout, stderr, c, commChan)
	return containerId, err
}

func (c *ScenarioOrchestrator) RunGraph(scenarios orchestrator_models.ScenarioSet, resolvedGraph orchestrator_models.ResolvedGraph, containerRuntimeUri string, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *orchestrator_models.GraphCommChannel) {
	scenario_orchestrator.CommonRunGraph(scenarios, resolvedGraph, containerRuntimeUri, extraEnv, extraVolumeMounts, cache, commChannel, c, c.Config)
}

func (c *ScenarioOrchestrator) PrintContainerRuntime() {
	scenario_orchestrator.CommonPrintRuntime(c.ContainerRuntime)
}

func (c *ScenarioOrchestrator) ListRunningScenarios(containerRuntimeUri string) (*[]orchestrator_models.RunningScenario, error) {
	return scenario_orchestrator.CommonListRunningScenarios(containerRuntimeUri, c)
}
