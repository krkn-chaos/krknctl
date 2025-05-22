// Package scenarioorchestrator provides the interface and the common function to interact with the underlying container environment
package scenarioorchestrator

import (
	"context"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	orchestrator_models "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"io"
	"os"
)

type ScenarioOrchestrator interface {
	Connect(containerRuntimeURI string) (context.Context, error)

	Run(
		image string,
		containerName string,
		env map[string]string,
		cache bool,
		volumeMounts map[string]string,
		commChan *chan *string,
		ctx context.Context,
		registry *models.RegistryV2,
	) (*string, error)

	RunAttached(
		image string,
		containerName string,
		env map[string]string,
		cache bool,
		volumeMounts map[string]string,
		stdout io.Writer,
		stderr io.Writer,
		commChan *chan *string,
		ctx context.Context,
		registry *models.RegistryV2,
	) (*string, error)

	RunGraph(
		scenarios orchestrator_models.ScenarioSet,
		resolvedGraph orchestrator_models.ResolvedGraph,
		extraEnv map[string]string,
		extraVolumeMounts map[string]string,
		cache bool,
		commChannel chan *orchestrator_models.GraphCommChannel,
		registry *models.RegistryV2,
		userID *int,
	)

	CleanContainers(ctx context.Context) (*int, error)

	AttachWait(
		containerID *string,
		stdout io.Writer,
		stderr io.Writer,
		ctx context.Context,
	) (*bool, error)

	Attach(
		containerID *string,
		signalChannel chan os.Signal,
		stdout io.Writer,
		stderr io.Writer,
		ctx context.Context,
	) (bool, error)

	Kill(containerID *string, ctx context.Context) error

	ListRunningContainers(ctx context.Context) (*map[int64]orchestrator_models.Container, error)
	ListRunningScenarios(ctx context.Context) (*[]orchestrator_models.ScenarioContainer, error)
	InspectScenario(container orchestrator_models.Container, ctx context.Context) (*orchestrator_models.ScenarioContainer, error)

	GetContainerRuntimeSocket(userID *int) (*string, error)

	GetContainerRuntime() orchestrator_models.ContainerRuntime
	PrintContainerRuntime()
	GetConfig() config.Config
	ResolveContainerName(containerName string, ctx context.Context) (*string, error)
}
