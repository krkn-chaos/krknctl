package scenarioorchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"sort"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/resiliency"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
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
	// collectors initialization
	var (
		allReports []resiliency.DetailedScenarioReport
		reportsMu  sync.Mutex
	)

	for step, s := range resolvedGraph {
		var wg sync.WaitGroup
		for _, scID := range s {
			scCopy := scenarios[scID]

			socket, err := orchestrator.GetContainerRuntimeSocket(userID)
			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioID: nil, ScenarioLogFile: nil, Err: err}
				return
			}

			ctx, err := orchestrator.Connect(*socket)
			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioID: nil, ScenarioLogFile: nil, Err: err}
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

			// set RESILIENCY_ENABLED_MODE using shared utility
			promURL := ""
			if prom, ok := env["PROMETHEUS_URL"]; ok {
				promURL = prom
			}
			env[config.EnvResiliencyEnabledMode] = resiliency.ComputeResiliencyMode(promURL, config)

			// inject resiliency config as base64 encoded env variable if provided (Use Case: To pass the custom resiliency config to the container)
			if scenario.ResiliencyConfigPath != "" {
				if content, err := os.ReadFile(scenario.ResiliencyConfigPath); err == nil {
					env[config.EnvKrknAlertsYamlContent] = base64.StdEncoding.EncodeToString(content)
				}
			}

			containerName := utils.GenerateContainerName(config, scenario.Name, &scID)
			filename := fmt.Sprintf("%s.log", containerName)
			file, err := os.Create(path.Clean(filename))

			if err != nil {
				commChannel <- &models.GraphCommChannel{Layer: nil, ScenarioID: nil, ScenarioLogFile: nil, Err: err}
				return
			}
			commChannel <- &models.GraphCommChannel{Layer: &step, ScenarioID: &scID, ScenarioLogFile: &filename, Err: nil}
			wg.Add(1)

			go func(scenario models.Scenario) {
				defer wg.Done()
				mw := io.MultiWriter(os.Stdout, file)
				_, err = orchestrator.RunAttached(scenario.Image, containerName, env, cache, volumes, mw, mw, nil, ctx, registry)
				_ = file.Sync()
				_ = file.Close()

				if data, readErr := os.ReadFile(filename); readErr == nil {
					if rep, parseErr := resiliency.ParseResiliencyReport(data); parseErr == nil {
						// Attach weight information for this scenario.
						weight := scenario.ResiliencyWeight
						if weight <= 0 {
							weight = 1
						}
						if rep.ScenarioWeights == nil {
							rep.ScenarioWeights = make(map[string]float64)
						}
						rep.ScenarioWeights[scenario.Name] = weight

						fmt.Fprintf(os.Stderr, "Parsed resiliency report from %s\n", filename)
						reportsMu.Lock()
						allReports = append(allReports, *rep)
						reportsMu.Unlock()
					} else {
						fmt.Fprintf(os.Stderr, "Failed to parse resiliency report from %s: %v\n", filename, parseErr)
					}
				}

				if err != nil {
					commChannel <- &models.GraphCommChannel{Layer: &step, ScenarioID: &scID, ScenarioLogFile: &filename, Err: err}
					return
				}
			}(scCopy.Scenario)

		}
		wg.Wait()

	}

	if err := resiliency.GenerateAndWriteReport(allReports, "resiliency-report.json"); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating resiliency report: %v\n", err)
	} else {
		fmt.Println("Detailed resiliency report written to resiliency-report.json")
	}

	commChannel <- nil
}

func CommonRunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, c ScenarioOrchestrator, commChan *chan *string, ctx context.Context, registry *providermodels.RegistryV2) (*string, error) {

	containerID, err := c.Run(image, containerName, env, cache, volumeMounts, commChan, ctx, registry)
	if err != nil {
		return nil, err
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	kill, err := c.Attach(containerID, signalChan, stdout, stderr, ctx)
	if err != nil {
		return containerID, err
	}

	if kill {
		if err := c.Kill(containerID, ctx); err != nil {
			return containerID, fmt.Errorf("failed to kill container: %w", err)
		}
	}

	containerStatus, err := c.InspectScenario(models.Container{ID: *containerID}, ctx)
	if err != nil {
		return containerID, fmt.Errorf("failed to inspect container: %w", err)
	}

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

func CommonAttachWait(containerID *string, stdout io.Writer, stderr io.Writer, c ScenarioOrchestrator, ctx context.Context) (bool, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	interrupted, err := c.Attach(containerID, sigCh, stdout, stderr, ctx)
	return interrupted, err
}

func CommonListRunningScenarios(c ScenarioOrchestrator, ctx context.Context) (*[]models.ScenarioContainer, error) {
	containersMap, err := c.ListRunningContainers(ctx)
	if err != nil {
		return nil, err
	}

	var indexes []int64
	var runningScenarios []models.ScenarioContainer

	for k := range *containersMap {
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
