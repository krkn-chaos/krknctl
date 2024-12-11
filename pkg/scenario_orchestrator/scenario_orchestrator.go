package scenario_orchestrator

import (
	"context"
	"github.com/krkn-chaos/krknctl/pkg/config"
	orchestrator_models "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"io"
	"os"
)

type ScenarioOrchestrator interface {
	Connect(containerRuntimeUri string) (context.Context, error)

	Run(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, commChan *chan *string, ctx context.Context, debug bool) (*string, error)

	RunAttached(image string, containerName string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer, commChan *chan *string, ctx context.Context, debug bool) (*string, error)

	RunGraph(scenarios orchestrator_models.ScenarioSet, resolvedGraph orchestrator_models.ResolvedGraph, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *orchestrator_models.GraphCommChannel, ctx context.Context, debug bool)

	CleanContainers(ctx context.Context) (*int, error)

	AttachWait(containerId *string, stdout io.Writer, stderr io.Writer, ctx context.Context) (*bool, error)

	Attach(containerId *string, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer, ctx context.Context) (bool, error)

	Kill(containerId *string, ctx context.Context) error

	ListRunningContainers(ctx context.Context) (*map[int64]orchestrator_models.Container, error)
	ListRunningScenarios(ctx context.Context) (*[]orchestrator_models.ScenarioContainer, error)
	InspectScenario(container orchestrator_models.Container, ctx context.Context) (*orchestrator_models.ScenarioContainer, error)

	GetContainerRuntimeSocket(userId *int) (*string, error)

	GetContainerRuntime() orchestrator_models.ContainerRuntime
	PrintContainerRuntime()
	GetConfig() config.Config
	ResolveContainerName(containerName string, ctx context.Context) (*string, error)
}
