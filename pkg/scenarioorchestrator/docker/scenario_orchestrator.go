package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/krkn-chaos/krknctl/pkg/config"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type ScenarioOrchestrator struct {
	Config           config.Config
	ContainerRuntime orchestratormodels.ContainerRuntime
}

func (c *ScenarioOrchestrator) Run(
	image string,
	containerName string,
	env map[string]string,
	cache bool,
	volumeMounts map[string]string,
	commChan *chan *string,
	ctx context.Context,
	registry *providermodels.RegistryV2,
) (*string, error) {

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	exists, err := ImageExists(ctx, cli, image, "")
	if err != nil {
		return nil, err
	}
	if !cache || !exists {
		err := pullImage(ctx, cli, image, commChan, registry)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	if err := cli.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return nil, err
	}

	return &resp.ID, nil
}

func (c *ScenarioOrchestrator) Attach(containerID *string, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer, ctx context.Context) (bool, error) {
	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return false, err
	}
	options := dockercontainer.LogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Follow:     true,
	}

	reader, err := cli.ContainerLogs(context.Background(), *containerID, options)
	if err != nil {
		return false, err
	}
	var deferErr error = nil
	defer func() {
		deferErr = reader.Close()
	}()
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
		respCh, errCH := cli.ContainerWait(ctx, *containerID, dockercontainer.WaitConditionNotRunning)
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
		return false, deferErr
	}

}

func (c *ScenarioOrchestrator) Kill(containerID *string, ctx context.Context) error {
	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return err
	}

	err = cli.ContainerKill(ctx, *containerID, "KILL")
	if err != nil {
		return err
	}
	return nil
}

func (c *ScenarioOrchestrator) CleanContainers(ctx context.Context) (*int, error) {
	nameRegex, err := regexp.Compile(fmt.Sprintf("%s.*-[0-9]+$", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}
	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	containerCount := 0
	for _, container := range containers {
		if container.State != "running" && nameRegex.MatchString(container.Names[0]) {
			err := cli.ContainerRemove(ctx, container.ID, dockercontainer.RemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			})
			if err != nil {
				return nil, err
			} else {
				containerCount++
			}
		}
	}
	return &containerCount, nil
}

func (c *ScenarioOrchestrator) GetContainerRuntimeSocket(userID *int) (*string, error) {
	return utils.GetSocketByContainerEnvironment(orchestratormodels.Docker, c.Config, userID)
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

	imgs, err := cli.ImageList(ctx, dockerimage.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, img := range imgs {
		if len(img.RepoTags) > 0 && img.RepoTags[0] == imageName {
			return true, nil
		}
	}
	return false, nil
}

func pullImage(
	ctx context.Context,
	cli *client.Client,
	imageName string,
	commChan *chan *string,
	registry *providermodels.RegistryV2,
) error {

	pullOptions := dockerimage.PullOptions{}
	if registry != nil {
		registryAuth, err := registry.ToDockerV2AuthString()
		if err != nil {
			return err
		}
		pullOptions.RegistryAuth = *registryAuth
	}
	reader, err := cli.ImagePull(ctx, imageName, pullOptions)
	if err != nil {
		return err
	}
	var deferErr error
	defer func() {
		deferErr = reader.Close()
	}()

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

	return deferErr
}

func (c *ScenarioOrchestrator) GetContainerRuntime() orchestratormodels.ContainerRuntime {
	return c.ContainerRuntime
}

func (c *ScenarioOrchestrator) ListRunningContainers(ctx context.Context) (*map[int64]orchestratormodels.Container, error) {
	scenarios := make(map[int64]orchestratormodels.Container)
	scenarioNameRegex, err := regexp.Compile(fmt.Sprintf("%s-.*-([0-9]+)", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		if container.State == c.Config.DockerRunningState {
			groups := scenarioNameRegex.FindStringSubmatch(container.Names[0])
			if len(groups) > 1 {
				index, err := strconv.ParseInt(groups[1], 10, 64)
				if err != nil {
					return nil, err
				}
				containerName := strings.Replace(container.Names[0], "/", "", 1)
				scenarios[index] = orchestratormodels.Container{
					Name:    containerName,
					ID:      container.ID,
					Image:   container.Image,
					Started: index,
				}
			}
		}
	}
	return &scenarios, nil

}

func (c *ScenarioOrchestrator) InspectScenario(container orchestratormodels.Container, ctx context.Context) (*orchestratormodels.ScenarioContainer, error) {
	runningScenario := &orchestratormodels.ScenarioContainer{}
	scenario := orchestratormodels.Scenario{}
	scenario.Volumes = make(map[string]string)
	scenario.Env = make(map[string]string)
	runningScenario.Container = &container

	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inspectData, err := cli.ContainerInspect(ctx, container.ID)
	if err != nil {
		if strings.Contains(err.Error(), "No such container") {
			return nil, nil
		}
		return nil, err
	}
	container.Name = strings.Replace(inspectData.Name, "/", "", 1)
	container.Status = inspectData.State.Status
	container.ExitStatus = inspectData.State.ExitCode

	scenarioDetail := providermodels.ScenarioDetail{}
	scenarioDetail.Digest = &inspectData.ContainerJSONBase.Image
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

func (c *ScenarioOrchestrator) Connect(containerRuntimeURI string) (context.Context, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(containerRuntimeURI))
	if err != nil {
		return nil, err
	}

	ctxWithClient := contextWithDockerClient(ctx, cli)
	return ctxWithClient, nil
}

func (c *ScenarioOrchestrator) GetConfig() config.Config {
	return c.Config
}

func (c *ScenarioOrchestrator) ResolveContainerName(containerName string, ctx context.Context) (*string, error) {
	cli, err := dockerClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		if strings.Contains(container.Names[0], containerName) {
			return &container.ID, nil
		}
	}
	return nil, nil
}

// common functions

func (c *ScenarioOrchestrator) AttachWait(containerID *string, stdout io.Writer, stderr io.Writer, ctx context.Context) (*bool, error) {

	interrupted, err := scenario_orchestrator.CommonAttachWait(containerID, stdout, stderr, c, ctx)
	if err != nil {
		return nil, err
	}

	return &interrupted, nil
}

func (c *ScenarioOrchestrator) RunAttached(
	image string,
	containerName string,
	env map[string]string,
	cache bool,
	volumeMounts map[string]string,
	stdout io.Writer,
	stderr io.Writer,
	commChan *chan *string,
	ctx context.Context,
	registry *providermodels.RegistryV2,
) (*string, error) {
	containerID, err := scenario_orchestrator.CommonRunAttached(image, containerName, env, cache, volumeMounts, stdout, stderr, c, commChan, ctx, registry)
	return containerID, err
}

func (c *ScenarioOrchestrator) RunGraph(
	scenarios orchestratormodels.ScenarioSet,
	resolvedGraph orchestratormodels.ResolvedGraph,
	extraEnv map[string]string,
	extraVolumeMounts map[string]string,
	cache bool,
	commChannel chan *orchestratormodels.GraphCommChannel,
	registry *providermodels.RegistryV2,
	userID *int,
) {
	scenario_orchestrator.CommonRunGraph(scenarios, resolvedGraph, extraEnv, extraVolumeMounts, cache, commChannel, c, c.Config, registry, userID)
}

func (c *ScenarioOrchestrator) PrintContainerRuntime() {
	scenarioorchestrator.CommonPrintRuntime(c.ContainerRuntime)
}

func (c *ScenarioOrchestrator) ListRunningScenarios(ctx context.Context) (*[]orchestratormodels.ScenarioContainer, error) {
	return scenarioorchestrator.CommonListRunningScenarios(c, ctx)
}
