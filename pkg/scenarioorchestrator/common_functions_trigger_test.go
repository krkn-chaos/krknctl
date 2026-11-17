package scenarioorchestrator

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/config"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	orchestratorutils "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
)

type graphRunCall struct {
	image    string
	env      map[string]string
	registry *providermodels.RegistryV2
}

type graphRunOrchestratorStub struct {
	ScenarioOrchestrator
	mu          sync.Mutex
	calls       []graphRunCall
	triggerFail bool
	started     chan string
	release     chan struct{}
	active      int
	maxActive   int
}

func (o *graphRunOrchestratorStub) GetContainerRuntimeSocket(*int) (*string, error) {
	socket := "unix:///tmp/krknctl-test.sock"
	return &socket, nil
}

func (o *graphRunOrchestratorStub) Connect(string) (context.Context, error) {
	return context.Background(), nil
}

func (o *graphRunOrchestratorStub) RunAttached(image, _ string, env map[string]string, _ bool, _ map[string]string, _ io.Writer, _ io.Writer, _ *chan *string, _ context.Context, registry *providermodels.RegistryV2, _ []string, _ *PodmanCreateOptions, _ bool) (*string, error) {
	copiedEnv := make(map[string]string, len(env))
	for key, value := range env {
		copiedEnv[key] = value
	}
	o.mu.Lock()
	o.calls = append(o.calls, graphRunCall{image: image, env: copiedEnv, registry: registry})
	o.active++
	if o.active > o.maxActive {
		o.maxActive = o.active
	}
	o.mu.Unlock()
	if o.started != nil {
		o.started <- image
	}
	if o.release != nil {
		<-o.release
	}
	o.mu.Lock()
	o.active--
	o.mu.Unlock()
	if o.triggerFail && strings.HasSuffix(image, "/krkn-hub-multiarch:trigger") {
		return nil, errors.New("trigger timed out")
	}
	id := "mock-container"
	return &id, nil
}

func TestCommonRunGraphLimitsParallelismToResolvedLayerWidth(t *testing.T) {
	t.Chdir(t.TempDir())
	orchestrator := &graphRunOrchestratorStub{
		started: make(chan string, 5),
		release: make(chan struct{}, 5),
	}
	scenarios := models.ScenarioSet{}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		scenarios[id] = models.ScenarioNode{Scenario: models.Scenario{Name: id, Image: id}}
	}
	messages := make(chan *models.GraphCommChannel, 8)
	done := make(chan struct{})
	go func() {
		CommonRunGraph(scenarios, models.ResolvedGraph{{"a", "b"}, {"c", "d"}, {"e"}}, nil, nil, false, messages, orchestrator, config.Config{}, nil, nil, true)
		close(done)
	}()

	for range 2 {
		<-orchestrator.started
	}
	for range 2 {
		orchestrator.release <- struct{}{}
	}
	for range 2 {
		<-orchestrator.started
	}
	for range 2 {
		orchestrator.release <- struct{}{}
	}
	<-orchestrator.started
	orchestrator.release <- struct{}{}
	<-done

	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()
	if orchestrator.maxActive != 2 {
		t.Fatalf("maximum concurrent scenarios = %d, want resolved layer width 2", orchestrator.maxActive)
	}
}

func TestCommonRunGraphPreservesFixedSingleNodeExecutionHistory(t *testing.T) {
	for run := 0; run < 2; run++ {
		t.Chdir(t.TempDir())
		orchestrator := &graphRunOrchestratorStub{}
		scenarios := models.ScenarioSet{}
		for _, id := range []string{"seed-root", "seed-middle", "seed-leaf"} {
			scenarios[id] = models.ScenarioNode{Scenario: models.Scenario{Name: id, Image: id}}
		}
		messages := make(chan *models.GraphCommChannel, 8)
		CommonRunGraph(scenarios, models.ResolvedGraph{{"seed-root"}, {"seed-middle"}, {"seed-leaf"}}, nil, nil, false, messages, orchestrator, config.Config{}, nil, nil, true)
		got := make([]string, len(orchestrator.calls))
		for i, call := range orchestrator.calls {
			got[i] = call.image
		}
		want := []string{"seed-root", "seed-middle", "seed-leaf"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d execution history = %v, want graph order %v", run+1, got, want)
		}
	}
}

func TestCommonRunGraphRunsTriggerImageAndThenDependentNode(t *testing.T) {
	t.Chdir(t.TempDir())
	registry := &providermodels.RegistryV2{RegistryURL: "registry.example.test"}
	orchestrator := &graphRunOrchestratorStub{}
	triggerURL := "https://example.test/health"
	triggerConfig := config.Config{QuayHost: "quay.io", QuayOrg: "krkn-chaos", QuayScenarioRegistry: "krkn-hub-multiarch"}
	triggerParent := "prepare"
	dependentParent := "wait-for-event"
	scenarios := models.ScenarioSet{
		"prepare": {Scenario: models.Scenario{Name: "dummy-scenario"}},
		"wait-for-event": {
			Scenario: models.Scenario{
				NodeType: models.NodeTypeTrigger,
				Name:     "trigger",
				Image:    "caller-supplied-image-should-be-ignored",
				Env:      map[string]string{"TRIGGER_HTTP_URL": triggerURL},
			},
			Parent: &triggerParent,
		},
		"after-event": {
			Scenario: models.Scenario{Name: "dummy-scenario", Image: "registry.example.test/dummy-scenario"},
			Parent:   &dependentParent,
		},
	}
	messages := make(chan *models.GraphCommChannel, 8)

	CommonRunGraph(scenarios, models.ResolvedGraph{{"prepare"}, {"wait-for-event"}, {"after-event"}}, nil, nil, false, messages, orchestrator, triggerConfig, registry, nil, true)

	if len(orchestrator.calls) != 3 {
		t.Fatalf("RunAttached call count = %d, want 3", len(orchestrator.calls))
	}
	triggerCall := orchestrator.calls[1]
	if triggerCall.image != "quay.io/krkn-chaos/krkn-hub-multiarch:trigger" {
		t.Fatalf("trigger image = %q, want configured image", triggerCall.image)
	}
	if triggerCall.registry != nil {
		t.Fatal("trigger image should use the public registry path, got a private registry")
	}
	if triggerCall.env["TRIGGER_HTTP_URL"] != triggerURL {
		t.Fatalf("trigger env value = %q, want %q", triggerCall.env["TRIGGER_HTTP_URL"], triggerURL)
	}
	if got := orchestrator.calls[2].image; got != "registry.example.test/dummy-scenario" {
		t.Fatalf("dependent node image = %q, want its scenario image", got)
	}
	if orchestrator.calls[2].registry != registry {
		t.Fatal("dependent scenario should retain the graph registry")
	}
}

func TestCommonRunGraphSkipsDependentNodesAfterTriggerFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	orchestrator := &graphRunOrchestratorStub{triggerFail: true}
	triggerParent := "wait-for-event"
	scenarios := models.ScenarioSet{
		"wait-for-event": {
			Scenario: models.Scenario{NodeType: models.NodeTypeTrigger, Name: "trigger"},
		},
		"after-event": {
			Scenario: models.Scenario{Name: "dummy-scenario", Image: "registry.example.test/dummy-scenario"},
			Parent:   &triggerParent,
		},
	}
	messages := make(chan *models.GraphCommChannel, 8)

	triggerConfig := config.Config{QuayHost: "quay.io", QuayOrg: "krkn-chaos", QuayScenarioRegistry: "krkn-hub-multiarch"}
	CommonRunGraph(scenarios, models.ResolvedGraph{{"wait-for-event"}, {"after-event"}}, nil, nil, false, messages, orchestrator, triggerConfig, nil, nil, true)

	if len(orchestrator.calls) != 1 {
		t.Fatalf("RunAttached call count = %d, want only the trigger node", len(orchestrator.calls))
	}
	if orchestrator.calls[0].image != "quay.io/krkn-chaos/krkn-hub-multiarch:trigger" {
		t.Fatalf("failed node image = %q, want configured image", orchestrator.calls[0].image)
	}
	var sawSkippedDependent bool
	for {
		message := <-messages
		if message == nil {
			break
		}
		if message.Err != nil && strings.Contains(message.Err.Error(), `skipping node "after-event"`) {
			var exitErr *orchestratorutils.ExitError
			if errors.As(message.Err, &exitErr) {
				t.Fatal("skipped node error should not be reported as a scenario exit error")
			}
			sawSkippedDependent = true
		}
	}
	if !sawSkippedDependent {
		t.Fatal("dependent node was not reported as skipped after trigger failure")
	}
}
