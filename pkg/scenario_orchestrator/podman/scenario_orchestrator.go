package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types/mount"
	"github.com/krkn-chaos/krknctl/internal/config"
	provider_models "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator"
	orchestrator_models "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/opencontainers/runtime-spec/specs-go"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ScenarioOrchestrator struct {
	Config           config.Config
	ContainerRuntime orchestrator_models.ContainerRuntime
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

func (c *ScenarioOrchestrator) Run(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, commChan *chan *string) (*string, *context.Context, error) {
	conn, err := c.Connect(containerRuntimeUri)
	if err != nil {
		return nil, nil, err
	}
	//if the image exists but the digest has changed pulls the image again
	imageExists, err := images.Exists(conn, image, nil)
	if cache == false || imageExists == false {

		// add a channel to update the status (eventually)
		progressChan := make(chan ProgressMessage)
		errChan := make(chan error)
		go func() {
			writer := &progressWriter{channel: progressChan}
			options := images.PullOptions{}
			options.WithProgressWriter(writer)
			_, err = images.Pull(conn, image, &options)
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
			return nil, nil, err
		}
	}

	if err != nil {
		return nil, nil, err
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
	createResponse, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		return nil, nil, err
	}
	return &createResponse.ID, &conn, nil
}

func (c *ScenarioOrchestrator) Attach(containerId *string, conn *context.Context, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer) (bool, error) {

	options := new(containers.AttachOptions).WithLogs(true).WithStream(true).WithDetachKeys("ctrl-c")

	errorChannel := make(chan error, 1)
	finishChannel := make(chan bool, 1)
	go func() {
		err := containers.Attach(*conn, *containerId, nil, stdout, stderr, nil, options)
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

func (c *ScenarioOrchestrator) CleanContainers() (*int, error) {
	_true := true
	nameRegex, err := regexp.Compile(fmt.Sprintf("^%s.*-[0-9]+$", c.Config.ContainerPrefix))
	if err != nil {
		return nil, err
	}
	socket, err := c.GetContainerRuntimeSocket(nil)
	if err != nil {
		return nil, err
	}
	conn, err := c.Connect(*socket)
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

func (c *ScenarioOrchestrator) Kill(containerId *string, ctx *context.Context) error {
	err := containers.Kill(*ctx, *containerId, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *ScenarioOrchestrator) GetContainerRuntimeSocket(userId *int) (*string, error) {
	return utils.GetSocketByContainerEnvironment(orchestrator_models.Podman, c.Config, userId)
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
	conn, err := c.Connect(containerRuntimeUri)
	if err != nil {
		return nil, err
	}
	containerList, err := containers.List(conn, nil)
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
	conn, err := c.Connect(containerRuntimeUri)
	runningScenario := &orchestrator_models.RunningScenario{}
	scenario := orchestrator_models.Scenario{}
	scenario.Volumes = make(map[string]string)
	scenario.Env = make(map[string]string)
	runningScenario.Container = &container
	if err != nil {
		return nil, err
	}
	inspectData, err := containers.Inspect(conn, container.Id, nil)
	if err != nil {
		return nil, err
	}
	if inspectData == nil {
		return nil, fmt.Errorf("container %s not found", container.Id)
	}

	if inspectData.Config == nil {
		return nil, fmt.Errorf("container %s has no config", container.Id)
	}
	scenarioDetail := provider_models.ScenarioDetail{}
	scenarioDetail.Digest = inspectData.ImageDigest
	imageAndTag := strings.Split(inspectData.ImageName, ":")
	// if the image does not contain a tag the tag becomes all the image
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

// common functions

func (c *ScenarioOrchestrator) RunAttached(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, commChan *chan *string) (*string, error) {
	time.Sleep(2)
	return scenario_orchestrator.CommonRunAttached(image, containerName, containerRuntimeUri, env, cache, volumeMounts, stdout, stderr, c, commChan)
}

func (c *ScenarioOrchestrator) AttachWait(containerId *string, conn *context.Context, stdout io.Writer, stderr io.Writer) (*bool, error) {

	interrupted, err := scenario_orchestrator.CommonAttachWait(containerId, conn, stdout, stderr, c)
	if err != nil {
		return nil, err
	}
	return &interrupted, nil
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
