package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	graphmodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

type triggerGraphProviderStub struct {
	tags              *[]providermodels.ScenarioTag
	globalEnvironment *providermodels.ScenarioDetail
	globalError       error
	globalCalls       []string
	scenarioCalls     int
}

func (p *triggerGraphProviderStub) GetRegistryImages(*providermodels.RegistryV2) (*[]providermodels.ScenarioTag, error) {
	return p.tags, nil
}

func (p *triggerGraphProviderStub) GetGlobalEnvironment(_ *providermodels.RegistryV2, scenario string) (*providermodels.ScenarioDetail, error) {
	p.globalCalls = append(p.globalCalls, scenario)
	return p.globalEnvironment, p.globalError
}

func (p *triggerGraphProviderStub) GetScenarioDetail(string, *providermodels.RegistryV2) (*providermodels.ScenarioDetail, error) {
	p.scenarioCalls++
	return nil, nil
}

func (p *triggerGraphProviderStub) GetImageSignatureStatus(context.Context, *providermodels.RegistryV2, providermodels.ScenarioTag) (verify.SignatureStatus, error) {
	return verify.SignatureUnknown, nil
}

func (p *triggerGraphProviderStub) ScaffoldScenarios([]string, bool, *providermodels.RegistryV2, bool, *provider.ScaffoldSeed) (*string, error) {
	return nil, nil
}

func TestValidateGraphScenarioInputTriggerNode(t *testing.T) {
	var nodes map[string]graphmodels.ScenarioNode
	if err := json.Unmarshal([]byte(`{
		"wait-for-event": {
			"nodeType": "trigger",
			"env": {"TRIGGER_HTTP_URL": "https://example.test/health"}
		}
	}`), &nodes); err != nil {
		t.Fatalf("unmarshal graph: %v", err)
	}

	if got := nodes["wait-for-event"].NodeType; got != graphmodels.NodeTypeTrigger {
		t.Fatalf("nodeType = %q, want %q", got, graphmodels.NodeTypeTrigger)
	}

	variable := "TRIGGER_HTTP_URL"
	globalTags := []providermodels.ScenarioTag{{Name: "dummy-scenario"}}
	triggerProvider := &triggerGraphProviderStub{
		tags: &globalTags,
		globalEnvironment: &providermodels.ScenarioDetail{
			Fields: []typing.InputField{{Variable: &variable, Type: typing.String}},
		},
	}
	results := make(chan *struct {
		name *string
		err  error
	}, 3)

	validateGraphScenarioInput(nil, nodes, results, nil, triggerProvider)
	progress := <-results
	if progress == nil || progress.err != nil || progress.name == nil || *progress.name != graphmodels.NodeTypeTrigger {
		t.Fatalf("trigger progress = %+v, want trigger node progress", progress)
	}
	if result := <-results; result != nil {
		t.Fatalf("validation result = %+v, want nil completion", result)
	}
	if triggerProvider.scenarioCalls != 0 {
		t.Fatalf("trigger validation looked up a scenario %d times, want 0", triggerProvider.scenarioCalls)
	}
	if len(triggerProvider.globalCalls) != 1 || triggerProvider.globalCalls[0] != config.TriggerNodeTag {
		t.Fatalf("global metadata lookups = %v, want only trigger tag %q", triggerProvider.globalCalls, config.TriggerNodeTag)
	}
}

func TestValidateGraphScenarioInputRejectsUnknownTriggerEnvironmentVariable(t *testing.T) {
	globalTags := []providermodels.ScenarioTag{{Name: "dummy-scenario"}}
	triggerProvider := &triggerGraphProviderStub{
		tags:              &globalTags,
		globalEnvironment: &providermodels.ScenarioDetail{},
	}
	nodes := map[string]graphmodels.ScenarioNode{
		"wait-for-event": {Scenario: graphmodels.Scenario{NodeType: graphmodels.NodeTypeTrigger, Env: map[string]string{"NOT_A_TRIGGER_PARAMETER": "value"}}},
	}
	results := make(chan *struct {
		name *string
		err  error
	}, 3)

	validateGraphScenarioInput(nil, nodes, results, nil, triggerProvider)
	<-results // progress notification
	result := <-results
	if result == nil || result.err == nil {
		t.Fatal("validation succeeded for an unknown trigger environment variable")
	}
}

func TestValidateGraphScenarioInputRejectsTriggerVolumes(t *testing.T) {
	globalTags := []providermodels.ScenarioTag{{Name: "dummy-scenario"}}
	triggerProvider := &triggerGraphProviderStub{
		tags:              &globalTags,
		globalEnvironment: &providermodels.ScenarioDetail{},
	}
	nodes := map[string]graphmodels.ScenarioNode{
		"wait-for-event": {
			Scenario: graphmodels.Scenario{
				NodeType: graphmodels.NodeTypeTrigger,
				Volumes:  map[string]string{"/host/secret": "/tmp/trigger-input"},
			},
		},
	}
	results := make(chan *struct {
		name *string
		err  error
	}, 3)

	validateGraphScenarioInput(nil, nodes, results, nil, triggerProvider)
	<-results // progress notification
	result := <-results
	if result == nil || result.err == nil {
		t.Fatal("validation succeeded for a trigger node with a volume mount")
	}
	if !strings.Contains(result.err.Error(), "does not support volume mounts") {
		t.Fatalf("validation error = %v, want unsupported volume mount error", result.err)
	}
	if triggerProvider.scenarioCalls != 0 {
		t.Fatalf("trigger validation looked up a scenario %d times, want 0", triggerProvider.scenarioCalls)
	}
}

func TestValidateGraphScenarioInputRejectsUnsupportedNodeType(t *testing.T) {
	nodes := map[string]graphmodels.ScenarioNode{
		"unknown": {Scenario: graphmodels.Scenario{NodeType: "unknown"}},
	}
	results := make(chan *struct {
		name *string
		err  error
	}, 1)

	validateGraphScenarioInput(nil, nodes, results, nil)
	result := <-results
	if result == nil || result.err == nil {
		t.Fatal("validation succeeded for an unsupported node type")
	}
}
