package randomgraph

import (
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"math/rand"
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
			index := rand.Intn(len(keys))
			selectedKeys = append(selectedKeys, keys[index])
			keys = append(keys[:index], keys[index+1:]...)
		}
	} else {
		selectedKeys = make([]string, len(keys))
		copy(selectedKeys, keys)
	}

	for len(selectedKeys) > 0 {
		var randomSteps []string
		parallelScenarios := rand.Intn(maxParallel) + 1

		if len(selectedKeys) < parallelScenarios {
			parallelScenarios = len(selectedKeys)
		}
		for i := 1; i <= parallelScenarios; i++ {
			randomKey := rand.Intn(len(selectedKeys))
			randomSteps = append(randomSteps, selectedKeys[randomKey])
			selectedKeys = append(selectedKeys[:randomKey], selectedKeys[randomKey+1:]...)
		}
		randomGraph = append(randomGraph, randomSteps)

	}

	return randomGraph
}
