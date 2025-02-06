package podman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/containers/podman/v5/pkg/specgen"

	"github.com/docker/docker/api/types/mount"
	"github.com/krkn-chaos/krknctl/pkg/config"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/opencontainers/runtime-spec/specs-go"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type ScenarioOrchestrator struct {
	Config           config.Config
	ContainerRuntime orchestratormodels.ContainerRuntime
}

type ProgressMessage struct {
	Status string
	Detail string
}

type progressWriter struct {
	channel chan<- ProgressMessage
}

func (w *progressWriter) Write(p []byte) (n int, err error) {
	message := string(p)
	parts := strings.SplitN(message, ":", 2)
	if len(parts) == 2 {
		status := strings.TrimSpace(parts[0])
		detail := strings.TrimSpace(parts[1])
		w.channel <- ProgressMessage{Status: status, Detail: detail}
	}
	return len(p), nil
}

func (c *ScenarioOrchestrator) Run(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, commChan *chan *string, ctx context.Context, registry *providermodels.RegistryV2) (*string, error) {
	imageExists, err := images.Exists(ctx, image, nil)
	if cache == false || imageExists == false {

		// add a channel to update the status (eventually)
		progressChan := make(chan ProgressMessage)
		errChan := make(chan error)
		go func() {
			writer := &progressWriter{channel: progressChan}
			options := images.PullOptions{}
			if registry != nil {
				if registry.Token != nil {
					errChan <- fmt.Errorf("token authentication not yet supported in podman")
					close(progressChan)
					close(errChan)
					return
				}
				if registry.Username != nil {
					options.Username = registry.Username
				}
				if registry.Password != nil {
					options.Password = registry.Password
				}
				skipTls := !registry.UseTLS
				options.SkipTLSVerify = &skipTls
			}
			options.WithProgressWriter(writer)
			_, err = images.Pull(ctx, image, &options)
			if err != nil {
				errChan <- err
			}
			close(progressChan)
			close(errChan)
		}()
		go func() {
			for msg := range progressChan {
				if commChan != nil {
					message := fmt.Sprintf("Status: %s - %s", msg.Status, msg.Detail)
					*commChan <- &message
				}

			}
			if commChan != nil {
				close(*commChan)
			}

		}()

		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}
	s := specgen.NewSpecGenerator(image, false)

	s.Name = containerName
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
	createResponse, err := containers.CreateWithSpec(ctx, s, nil)
	if err != nil {
		return nil, err
	}
	if err := containers.Start(ctx, createResponse.ID, nil); err != nil {
		return nil, err
	}
	return &createResponse.ID, nil
}

func (c *ScenarioOrchestrator) Attach(containerId *string, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer, ctx context.Context) (bool, error) {

	options := new(containers.AttachOptions).WithLogs(true).WithStream(true).WithDetachKeys("ctrl-c")

	errorChannel := make(chan error, 1)
	finishChannel := make(chan bool, 1)
	var mu sync.Mutex
	go func() {
		mu.Lock()
		defer mu.Unlock()
		err := containers.Attach(ctx, *containerId, nil, stdout, stderr, nil, options)
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

func (c *ScenarioOrchestrator) CleanContainers(ctx context.Context) (*int, error) {
	_true := true
	nameRegex, err := regexp.Compile(fmt.Sprintf("^%s.*-[0-9]+$", c.Config.ContainerPrefix))
	foundContainers, err := containers.List(ctx, &containers.ListOptions{
		All: &_true,
	})
	if err != nil {
		return nil, err
	}
	deletedContainers := 0

	for _, c := range foundContainers {
		for _, n := range c.Names {
			if nameRegex.MatchString(n) {
				_, err := containers.Remove(ctx, n, &containers.RemoveOptions{
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

func (c *ScenarioOrchestrator) Kill(containerId *string, ctx context.Context) error {
	err := containers.Kill(ctx, *containerId, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *ScenarioOrchestrator) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return utils.GetSocketByContainerEnvironment(orchestratormodels.Podman, c.Config, userId)
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
	containerList, err := containers.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, container := range containerList {
		if container.State == c.Config.PodmanRunningState && scenarioNameRegex.MatchString(container.Names[0]) {
			groups := scenarioNameRegex.FindStringSubmatch(container.Names[0])
			if groups != nil && len(groups) > 1 {
				index, err := strconv.ParseInt(groups[1], 10, 64)
				if err != nil {
					return nil, err
				}
				scenarios[index] = orchestratormodels.Container{
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

func (c *ScenarioOrchestrator) InspectScenario(container orchestratormodels.Container, ctx context.Context) (*orchestratormodels.ScenarioContainer, error) {
	runningScenario := &orchestratormodels.ScenarioContainer{}
	scenario := orchestratormodels.Scenario{}
	scenario.Volumes = make(map[string]string)
	scenario.Env = make(map[string]string)
	runningScenario.Container = &container

	inspectData, err := containers.Inspect(ctx, container.Id, nil)
	if err != nil {
		var customErr *errorhandling.ErrorModel
		if errors.As(err, &customErr) {
			if customErr.ResponseCode == 404 {
				return nil, nil
			}
		}
	}

	container.Name = inspectData.Name
	container.Status = inspectData.State.Status
	container.ExitStatus = inspectData.State.ExitCode

	if inspectData.Config == nil {
		return nil, fmt.Errorf("container %s has no config", container.Id)
	}
	scenarioDetail := providermodels.ScenarioDetail{}
	scenarioDetail.Digest = &inspectData.ImageDigest
	imageAndTag := strings.Split(inspectData.ImageName, ":")
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
	scenario.Image = inspectData.ImageName
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
	return bindings.NewConnection(context.Background(), containerRuntimeUri)
}

func (c *ScenarioOrchestrator) GetConfig() config.Config {
	return c.Config
}

func (c *ScenarioOrchestrator) ResolveContainerName(containerName string, ctx context.Context) (*string, error) {
	_true := true
	containerList, err := containers.List(ctx, &containers.ListOptions{
		All: &_true,
	})
	if err != nil {
		return nil, err
	}
	for _, container := range containerList {
		if strings.Contains(container.Names[0], containerName) {
			return &container.ID, nil
		}
	}
	return nil, nil
}

// common functions

func (c *ScenarioOrchestrator) RunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, commChan *chan *string, ctx context.Context, registry *providermodels.RegistryV2) (*string, error) {
	return scenario_orchestrator.CommonRunAttached(image, containerName, env, cache, volumeMounts, stdout, stderr, c, commChan, ctx, nil)
}

func (c *ScenarioOrchestrator) AttachWait(containerId *string, stdout io.Writer, stderr io.Writer, ctx context.Context) (*bool, error) {

	interrupted, err := scenario_orchestrator.CommonAttachWait(containerId, stdout, stderr, c, ctx)
	if err != nil {
		return nil, err
	}
	return &interrupted, nil
}

func (c *ScenarioOrchestrator) RunGraph(scenarios orchestratormodels.ScenarioSet, resolvedGraph orchestratormodels.ResolvedGraph, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *orchestratormodels.GraphCommChannel, ctx context.Context, registry *providermodels.RegistryV2) {
	//TODO: add a getconfig method in scenarioOrchestrator
	scenario_orchestrator.CommonRunGraph(scenarios, resolvedGraph, extraEnv, extraVolumeMounts, cache, commChannel, c, c.Config, ctx, nil)
}

func (c *ScenarioOrchestrator) PrintContainerRuntime() {
	scenario_orchestrator.CommonPrintRuntime(c.ContainerRuntime)
}

func (c *ScenarioOrchestrator) ListRunningScenarios(ctx context.Context) (*[]orchestratormodels.ScenarioContainer, error) {
	return scenario_orchestrator.CommonListRunningScenarios(c, ctx)
}
