package scenario_orchestrator

import (
	"context"
	orchestrator_models "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"io"
	"os"
)

type ScenarioOrchestrator interface {
	Connect(containerRuntimeUri string) (context.Context, error)

	Run(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string) (*string, *context.Context, error)

	RunAttached(image string, containerName string, containerRuntimeUri string, env map[string]string, cache bool, volumeMounts map[string]string, stdout io.Writer, stderr io.Writer) (*string, error)

	RunGraph(scenarios orchestrator_models.ScenarioSet,
		resolvedGraph orchestrator_models.ResolvedGraph,
		containerRuntimeUri string,
		extraEnv map[string]string,
		extraVolumeMounts map[string]string,
		cache bool,
		commChannel chan *orchestrator_models.GraphCommChannel,
	)

	CleanContainers() (*int, error)

	AttachWait(containerId *string, ctx *context.Context, stdout io.Writer, stderr io.Writer) (*bool, error)

	Attach(containerId *string, ctx *context.Context, signalChannel chan os.Signal, stdout io.Writer, stderr io.Writer) (bool, error)

	Kill(containerId *string, ctx *context.Context) error

	ListRunningContainers(containerRuntimeUri string) (*map[int64]orchestrator_models.Container, error)
	ListRunningScenarios(containerRuntimeUri string) (*[]orchestrator_models.RunningScenario, error)
	InspectRunningScenario(container orchestrator_models.Container, containerRuntimeUri string) (*orchestrator_models.RunningScenario, error)

	GetContainerRuntimeSocket(userId *int) (*string, error)

	GetContainerRuntime() orchestrator_models.ContainerRuntime
	PrintContainerRuntime()
}
