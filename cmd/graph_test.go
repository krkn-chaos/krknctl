package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/stretchr/testify/require"
)

type graphCommandProviderFactory struct {
	provider provider.ScenarioDataProvider
}

func (f graphCommandProviderFactory) NewInstance(provider.Mode) provider.ScenarioDataProvider {
	return f.provider
}

type graphCommandProvider struct {
	signatureStatusProvider
	invalidScenario string
}

func (p graphCommandProvider) GetScenarioDetail(name string, registry *providermodels.RegistryV2) (*providermodels.ScenarioDetail, error) {
	if name == p.invalidScenario {
		return &providermodels.ScenarioDetail{}, nil
	}
	return p.details[name], p.errors[name]
}

func (p graphCommandProvider) GetGlobalEnvironment(*providermodels.RegistryV2, string) (*providermodels.ScenarioDetail, error) {
	return &providermodels.ScenarioDetail{}, nil
}

type graphCommandOrchestrator struct {
	MockScenarioOrchestrator

	mu          sync.Mutex
	plans       []orchestratormodels.ResolvedGraph
	history     []string
	active      int
	maxActive   int
	runGraphHit int
}

func (o *graphCommandOrchestrator) RunAttached(image, _ string, _ map[string]string, _ bool, _ map[string]string, _, _ io.Writer, _ *chan *string, _ context.Context, _ *providermodels.RegistryV2, _ []string, _ *scenarioorchestrator.PodmanCreateOptions, _ bool) (*string, error) {
	o.mu.Lock()
	o.active++
	if o.active > o.maxActive {
		o.maxActive = o.active
	}
	o.history = append(o.history, "start:"+image)
	o.mu.Unlock()

	time.Sleep(2 * time.Millisecond)

	o.mu.Lock()
	o.history = append(o.history, "finish:"+image)
	o.active--
	o.mu.Unlock()
	return stringPointer("container-id"), nil
}

func (o *graphCommandOrchestrator) RunGraph(scenarios orchestratormodels.ScenarioSet, resolvedGraph orchestratormodels.ResolvedGraph, extraEnv map[string]string, extraVolumeMounts map[string]string, cache bool, commChannel chan *orchestratormodels.GraphCommChannel, registry *providermodels.RegistryV2, userID *int, allowUnsigned bool) {
	o.mu.Lock()
	o.runGraphHit++
	o.plans = append(o.plans, cloneResolvedGraph(resolvedGraph))
	o.mu.Unlock()
	scenarioorchestrator.CommonRunGraph(scenarios, resolvedGraph, extraEnv, extraVolumeMounts, cache, commChannel, o, o.GetConfig(), registry, userID, allowUnsigned)
}

func cloneResolvedGraph(graph orchestratormodels.ResolvedGraph) orchestratormodels.ResolvedGraph {
	clone := make(orchestratormodels.ResolvedGraph, len(graph))
	for i := range graph {
		clone[i] = append([]string(nil), graph[i]...)
	}
	return clone
}

func stringPointer(value string) *string {
	return &value
}

func newGraphCommandTest(t *testing.T, invalidScenario string) (*graphCommandOrchestrator, error) {
	t.Helper()
	configValue := getConfig(t)
	providerValue := graphCommandProvider{
		signatureStatusProvider: signatureStatusProvider{details: map[string]*providermodels.ScenarioDetail{
			"root":    {ScenarioTag: providermodels.ScenarioTag{Name: "root"}, IsAScenario: true},
			"sibling": {ScenarioTag: providermodels.ScenarioTag{Name: "sibling"}, IsAScenario: true},
			"child":   {ScenarioTag: providermodels.ScenarioTag{Name: "child"}, IsAScenario: true},
		}},
		invalidScenario: invalidScenario,
	}
	if invalidScenario != "" {
		providerValue.details[invalidScenario] = &providermodels.ScenarioDetail{}
	}

	orchestrator := &graphCommandOrchestrator{}
	var orchestratorInterface scenarioorchestrator.ScenarioOrchestrator = orchestrator
	command := NewGraphRunCommand(graphCommandProviderFactory{provider: providerValue}, &orchestratorInterface, configValue)
	command.Flags().String("private-registry", "", "")
	command.Flags().String("private-registry-scenarios", "", "")
	command.Flags().String("private-registry-username", "", "")
	command.Flags().String("private-registry-password", "", "")
	command.Flags().String("private-registry-token", "", "")
	command.Flags().Bool("private-registry-insecure", false, "")
	command.Flags().Bool("private-registry-skip-tls", false, "")
	command.Flags().Bool("run-unsigned-images", true, "")
	command.Flags().String("kubeconfig", "", "")
	command.Flags().String("alerts-profile", "", "")
	command.Flags().String("metrics-profile", "", "")
	command.Flags().Bool("exit-on-error", false, "")
	command.Flags().StringArray("weight", nil, "")

	graphFile := filepath.Join(t.TempDir(), "graph.json")
	require.NoError(t, os.WriteFile(graphFile, []byte(`{
  "root": {"name": "root", "image": "image:root"},
  "sibling": {"name": "sibling", "image": "image:sibling", "depends_on": "root"},
  "child": {"name": "child", "image": "image:child", "depends_on": "root"}
}`), 0600))
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
clusters: []
contexts: []
current-context: ""
users: []
`), 0600))
	command.SetArgs([]string{graphFile, "--kubeconfig", kubeconfig})
	return orchestrator, command.Execute()
}

func TestGraphRunCommandPreservesValidationAndGraphWorkflow(t *testing.T) {
	first, err := newGraphCommandTest(t, "")
	require.NoError(t, err)
	second, err := newGraphCommandTest(t, "")
	require.NoError(t, err)

	first.mu.Lock()
	firstPlan := first.plans[0]
	firstHistory := append([]string(nil), first.history...)
	firstMaxActive := first.maxActive
	first.mu.Unlock()
	second.mu.Lock()
	secondPlan := second.plans[0]
	secondHistory := append([]string(nil), second.history...)
	second.mu.Unlock()
	sort.Strings(firstHistory)
	sort.Strings(secondHistory)

	require.Equal(t, firstPlan, secondPlan, "the same graph must produce the same ordered execution plan")
	require.Equal(t, firstHistory, secondHistory, "the same graph must produce the same execution history")
	require.Len(t, firstPlan, 2)
	require.Equal(t, []string{"root"}, firstPlan[0])
	require.ElementsMatch(t, []string{"child", "sibling"}, firstPlan[1])
	require.LessOrEqual(t, firstMaxActive, 2)
	require.Equal(t, 1, first.runGraphHit)

	invalidOrchestrator, err := newGraphCommandTest(t, "child")
	require.EqualError(t, err, `failed to validate scenario: child, error: selected scenario "child" is not a valid scenario (is_a_scenario=false)`)
	invalidOrchestrator.mu.Lock()
	defer invalidOrchestrator.mu.Unlock()
	require.Zero(t, invalidOrchestrator.runGraphHit, "workflow execution must not start after graph validation fails")
}

var _ scenarioProviderFactory = graphCommandProviderFactory{}
var _ scenarioorchestrator.ScenarioOrchestrator = (*graphCommandOrchestrator)(nil)
