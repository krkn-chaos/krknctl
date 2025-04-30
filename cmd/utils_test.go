package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDumpRandomGraph(t *testing.T) {
	data := `
{
  "a": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-cpu-hog",
    "name": "node-cpu-hog",
    "env": {
      "IMAGE": "quay.io/krkn-chaos/krkn-hog",
      "NAMESPACE": "default",
      "NODE_CPU_PERCENTAGE": "5",
      "NODE_SELECTOR": "node-role.kubernetes.io/worker=",
      "NUMBER_OF_NODES": "1",
      "TOTAL_CHAOS_DURATION": "10"
    }
  },
  "b": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-cpu-hog",
    "name": "node-cpu-hog",
    "env": {
      "IMAGE": "quay.io/krkn-chaos/krkn-hog",
      "NAMESPACE": "default",
      "NODE_CPU_PERCENTAGE": "5",
      "NODE_SELECTOR": "node-role.kubernetes.io/worker=",
      "NUMBER_OF_NODES": "1",
      "TOTAL_CHAOS_DURATION": "10"
    }
  },
  "c": {
    "image": "quay.io/krkn-chaos/krkn-hub:node-cpu-hog",
    "name": "node-cpu-hog",
    "env": {
      "IMAGE": "quay.io/krkn-chaos/krkn-hog",
      "NAMESPACE": "default",
      "NODE_CPU_PERCENTAGE": "5",
      "NODE_SELECTOR": "node-role.kubernetes.io/worker=",
      "NUMBER_OF_NODES": "1",
      "TOTAL_CHAOS_DURATION": "10"
    }
  }
}
`
	var unserializedNodes map[string]models.ScenarioNode
	err := json.Unmarshal([]byte(data), &unserializedNodes)
	assert.Nil(t, err)
	plan := [][]string{{"a"}, {"b", "c"}}

	graph := RebuildDependencyGraph(unserializedNodes, plan, "root")

	assert.Nil(t, graph["a"].Parent)
	assert.Equal(t, graph["a"].Comment, "root")
	assert.Equal(t, *graph["b"].Parent, "a")
	assert.Equal(t, *graph["c"].Parent, "a")

	fileName := fmt.Sprintf("graph-%d.json", time.Now().Unix())
	err = DumpRandomGraph(unserializedNodes, plan, fileName, "root")
	assert.Nil(t, err)
	assert.FileExists(t, fileName)

}
