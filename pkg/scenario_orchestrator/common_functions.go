package scenario_orchestrator

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/utils"
	"io"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
)

func CommonRunGraph(scenarios models.ScenarioSet, resolvedGraph models.ResolvedGraph, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *models.GraphCommChannel, orchestrator ScenarioOrchestrator, config config.Config, ctx context.Context, debug bool) {
	env := make(map[string]string)
	volumes := make(map[string]string)

	for k, v := range extraEnv {
		env[k] = v
	}

	for k, v := range extraVolumeMounts {
		volumes[k] = v
	}

	for step, s := range resolvedGraph {
		var wg sync.WaitGroup
		for _, scId := range s {
			scenario := scenarios[scId]
			for k, v := range scenario.Env {
				env[k] = v
			}
			for k, v := range scenario.Volumes {
				volumes[k] = v
			}

			containerName := utils.GenerateContainerName(config, scenario.Name, &scId)
			filename := fmt.Sprintf("%s.log", containerName)
			file, err := os.Create(filename)

			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioId: nil, ScenarioLogFile: nil, Err: err}
				return
			}
			commChannel <- &models.GraphCommChannel{Layer: &step, ScenarioId: &scId, ScenarioLogFile: &filename, Err: nil}
			wg.Add(1)

			go func() {
				defer wg.Done()
				_, _ = orchestrator.RunAttached(scenario.Image, containerName, env, cache, volumes, file, file, nil, ctx, debug)
			}()

		}
		wg.Wait()
	}
	commChannel <- nil
}

func CommonRunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, c ScenarioOrchestrator, commChan *chan *string, ctx context.Context, debug bool) (*string, error) {

	containerId, err := c.Run(image, containerName, env, cache, volumeMounts, commChan, ctx, debug)
	if err != nil {
		return nil, err
	}

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	kill, err := c.Attach(containerId, signalChan, stdout, stderr, ctx)

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

	containerStatus, err := c.InspectScenario(models.Container{Id: *containerId}, ctx)
	if err != nil {
		return nil, err
	}
	// if there is an error exit status it is propagated via error to the cmd
	if containerStatus.Container.ExitStatus > 0 {
		return containerId, utils.StatusCodeToError(containerStatus.Container.ExitStatus, c.GetConfig())
	}

	return containerId, nil
}

func CommonPrintRuntime(containerRuntime models.ContainerRuntime) {
	green := color.New(color.FgGreen).SprintFunc()
	boldGreen := color.New(color.FgHiGreen, color.Bold).SprintFunc()
	fmt.Printf("\n\n%s %s\n\n", green("container runtime:"), boldGreen(containerRuntime.String()))
}

func CommonAttachWait(containerId *string, stdout io.Writer, stderr io.Writer, c ScenarioOrchestrator, ctx context.Context) (bool, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	interrupted, err := c.Attach(containerId, sigCh, stdout, stderr, ctx)
	return interrupted, err
}

func CommonListRunningScenarios(c ScenarioOrchestrator, ctx context.Context) (*[]models.ScenarioContainer, error) {
	containersMap, err := c.ListRunningContainers(ctx)
	if err != nil {
		return nil, err
	}

	var indexes []int64
	var runningScenarios []models.ScenarioContainer

	for k, _ := range *containersMap {
		indexes = append(indexes, k)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })

	for _, index := range indexes {
		container := (*containersMap)[index]
		scenario, err := c.InspectScenario(container, nil)
		if err != nil {
			return nil, err
		}
		runningScenarios = append(runningScenarios, *scenario)

	}

	return &runningScenarios, nil
}
