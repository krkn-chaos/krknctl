package scenarioorchestrator

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"io"
	"os"
	"os/signal"
	"path"
	"sort"
	"sync"
	"syscall"
)

func CommonRunGraph(
	scenarios models.ScenarioSet,
	resolvedGraph models.ResolvedGraph,
	extraEnv map[string]string,
	extraVolumeMounts map[string]string,
	cache bool,
	commChannel chan *models.GraphCommChannel,
	orchestrator ScenarioOrchestrator,
	config config.Config,
	registry *providermodels.RegistryV2,
	userID *int,
) {
	for step, s := range resolvedGraph {
		var wg sync.WaitGroup
		for _, scID := range s {

			socket, err := orchestrator.GetContainerRuntimeSocket(userID)
			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioId: nil, ScenarioLogFile: nil, Err: err}
				return
			}

			ctx, err := orchestrator.Connect(*socket)
			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioId: nil, ScenarioLogFile: nil, Err: err}
				return
			}

			env := make(map[string]string)
			volumes := make(map[string]string)

			for k, v := range extraEnv {
				env[k] = v
			}

			for k, v := range extraVolumeMounts {
				volumes[k] = v
			}

			scenario := scenarios[scID]
			for k, v := range scenario.Env {
				env[k] = v
			}
			for k, v := range scenario.Volumes {
				volumes[k] = v
			}

			containerName := utils.GenerateContainerName(config, scenario.Name, &scID)
			filename := fmt.Sprintf("%s.log", containerName)
			file, err := os.Create(path.Clean(filename))

			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioId: nil, ScenarioLogFile: nil, Err: err}
				return
			}
			commChannel <- &models.GraphCommChannel{Layer: &step, ScenarioId: &scID, ScenarioLogFile: &filename, Err: nil}
			wg.Add(1)

			go func() {
				defer wg.Done()
				_, err = orchestrator.RunAttached(scenario.Image, containerName, env, cache, volumes, file, file, nil, ctx, registry)
				if err != nil {
					commChannel <- &models.GraphCommChannel{Layer: &step, ScenarioId: &scID, ScenarioLogFile: &filename, Err: err}
					return
				}
			}()

		}
		wg.Wait()
	}
	commChannel <- nil
}

func CommonRunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, c ScenarioOrchestrator, commChan *chan *string, ctx context.Context, registry *providermodels.RegistryV2) (*string, error) {

	containerID, err := c.Run(image, containerName, env, cache, volumeMounts, commChan, ctx, registry)
	if err != nil {
		return nil, err
	}

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	kill, err := c.Attach(containerID, signalChan, stdout, stderr, ctx)

	if err != nil {
		return nil, err
	}

	if kill {
		err = c.Kill(containerID, ctx)
		if err != nil {
			return nil, err
		}
		_, err = color.New(color.FgRed, color.Underline).Println(fmt.Sprintf("container %s killed", *containerID))
		if err != nil {
			return nil, err
		}
	}

	containerStatus, err := c.InspectScenario(models.Container{Id: *containerID}, ctx)
	if err != nil {
		return nil, err
	}
	// if there is an error exit status it is propagated via error to the cmd
	if containerStatus.Container.ExitStatus > 0 {
		return containerID, &utils.ExitError{ExitStatus: int(containerStatus.Container.ExitStatus)}
	}

	return containerID, nil
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
		scenario, err := c.InspectScenario(container, ctx)
		if err != nil {
			return nil, err
		}
		if scenario == nil {
			continue
		}
		runningScenarios = append(runningScenarios, *scenario)

	}

	return &runningScenarios, nil
}
