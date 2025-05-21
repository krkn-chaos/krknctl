package randomgraph

import (
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/utils"
)

func NewRandomGraph(nodes map[string]models.ScenarioNode, maxParallel int, numberOfScenarios int) [][]string {
	var randomGraph [][]string
	keys := make([]string, 0)
	selectedKeys := make([]string, 0)
	for key := range nodes {
		if key != "_comment" {
			keys = append(keys, key)
		}
	}

	if numberOfScenarios != 0 {
		for i := 0; i < numberOfScenarios; i++ {
			keysLen := len(keys)
			index := utils.RandomInt64(&keysLen)
			selectedKeys = append(selectedKeys, keys[index])
			keys = append(keys[:index], keys[index+1:]...)
		}
	} else {
		selectedKeys = make([]string, len(keys))
		copy(selectedKeys, keys)
	}

	for len(selectedKeys) > 0 {
		var randomSteps []string
		parallelScenarios := utils.RandomInt64(&maxParallel) + 1

		if int64(len(selectedKeys)) < parallelScenarios {
			parallelScenarios = int64(len(selectedKeys))
		}
		for i := int64(1); i <= parallelScenarios; i++ {
			lenSelectedKeys := len(selectedKeys)
			randomKey := utils.RandomInt64(&lenSelectedKeys)
			randomSteps = append(randomSteps, selectedKeys[randomKey])
			selectedKeys = append(selectedKeys[:randomKey], selectedKeys[randomKey+1:]...)
		}
		randomGraph = append(randomGraph, randomSteps)

	}

	return randomGraph
}
