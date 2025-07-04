package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	models2 "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/letsencrypt/boulder/core"
	"github.com/tjarratt/babble"
	"os"
	"strings"
)

func GetKrknctlLabel(label string, layers []ContainerLayer) *string {
	for _, v := range layers {
		commands := v.GetCommands()
		for _, c := range commands {
			if strings.Contains(c, fmt.Sprintf("LABEL %s", label)) {
				return &c
			}
		}
	}
	return nil
}

func ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, config config.Config, p ScenarioDataProvider, random bool, seed *ScaffoldSeed) (*string, error) {
	var scenarioNodes map[string]models2.ScenarioNode
	var err error
	babbler := babble.NewBabbler()
	// handles babble panic when american word dictionary is not installed
	defer func() {
		if err := recover(); err != nil {
			_, err = color.New(color.FgRed, color.Underline).Println("impossible to locate american words dictionary " +
				"please refer to the documentation on how to install this dependency https://github.com/krkn-chaos/krknctl#Requirements")
		}
	}()

	babbler.Count = 1
	if seed == nil {
		scenarioNodes, err = scaffoldScenarios(scenarios, includeGlobalEnv, registry, config, p, random, babbler)
		if err != nil {
			return nil, err
		}
	} else {
		scenarioNodes, err = scaffoldSeededScenarios(seed)
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(scenarioNodes)
	if err != nil {
		return nil, err
	}
	jsonBuf := buf.String()
	return &jsonBuf, nil
}

func scaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, config config.Config, p ScenarioDataProvider, random bool, babbler babble.Babbler) (map[string]models2.ScenarioNode, error) {
	var scenarioDetails []models.ScenarioDetail
	for _, scenarioName := range scenarios {
		scenarioDetail, err := p.GetScenarioDetail(scenarioName, registry)

		if err != nil {
			return nil, err
		}

		if scenarioDetail == nil {
			return nil, fmt.Errorf("scenario '%s' not found", scenarioName)
		}

		scenarioDetails = append(scenarioDetails, *scenarioDetail)
	}
	// builds all the indexes for the json upfront, so I can suggest the root node in the _comment
	var indexes []string
	for _, scenario := range scenarios {
		indexes = append(indexes, fmt.Sprintf("%s-%s", scenario, strings.ToLower(babbler.Babble())))
	}
	var scenarioNodes = make(map[string]models2.ScenarioNode)
	// if random is set _comment is not set
	if !random {
		scenarioNodes["_comment"] = GetInstructionScenario(indexes[0])
	}
	for i, scenarioDetail := range scenarioDetails {
		indexes = append(indexes, strings.ToLower(babbler.Babble()))

		scenarioNode := models2.ScenarioNode{}

		// if random is not set dependencies will be set
		if !random {
			if i > 0 {
				scenarioNode.Parent = &indexes[i-1]
			} else {
				scenarioNode.Comment = config.LabelRootNode
			}
		}

		var imageURI = ""
		if registry == nil {
			uri, err := config.GetCustomDomainImageURI()
			if err != nil {
				return nil, err
			}
			imageURI = uri
		} else {
			imageURI = registry.GetPrivateRegistryURI()
		}
		scenarioNode.Image = imageURI + ":" + scenarioDetail.Name
		scenarioNode.Name = scenarioDetail.Name
		scenarioNode.Env = make(map[string]string)
		scenarioNode.Volumes = make(map[string]string)

		for _, detail := range scenarioDetail.Fields {

			switch detail.Type {
			case typing.File:
				scenarioNode.Volumes[fmt.Sprintf("<put your local file_%d path here>", i)] = *detail.MountPath
			default:
				if detail.Default != nil && *detail.Default != "" {
					scenarioNode.Env[*detail.Variable] = *detail.Default
				} else {
					var required = ""
					if detail.Required {
						required = "(required)"
					}
					scenarioNode.Env[*detail.Variable] = fmt.Sprintf("<%s%s>", *detail.Description, required)
				}

			}

		}

		if includeGlobalEnv {
			globalDetail, err := p.GetGlobalEnvironment(registry, scenarios[0])
			if err != nil {
				return nil, err
			}

			for _, field := range globalDetail.Fields {
				scenarioNode.Env[*field.Variable] = *field.Default

			}
		}
		scenarioNodes[indexes[i]] = scenarioNode
	}

	return scenarioNodes, nil
}

func scaffoldSeededScenarios(seed *ScaffoldSeed) (map[string]models2.ScenarioNode, error) {
	var nodeMap map[string]models2.ScenarioNode
	resultMap := make(map[string]models2.ScenarioNode)
	buf, err := os.ReadFile(seed.Path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(buf, &nodeMap)
	for key := range nodeMap {
		if key == "_comment" {
			delete(nodeMap, key)
		}
	}
	if err != nil {
		return nil, err
	}
	total := int64(100)
	seedNumber := int64(len(nodeMap))
	if seedNumber > total {
		return nil, fmt.Errorf("seed file with more than %d nodes is not supported", total)
	}

	counter := int64(0)
	minimum := int64(1)
	percentage := int64(0)
	slot := (100 / seedNumber) + 10
	var percentages []int64
	var keys []string
	for key := range nodeMap {
		keys = append(keys, key)
		if counter == seedNumber-1 {
			// the last round gets all the remaining percentage
			// to ensure that the total will always be 100 even if
			// the rest of the division is greater than 0
			percentage = total
		} else {
			limit := slot - minimum + 1
			percentage = utils.RandomInt64(&limit) + minimum
		}

		percentages = append(percentages, percentage)

		total -= percentage
		// this sets the minimum for the next round with the remainder of the current round
		// to increase the probability that will be higher and the slot will be more filled
		minimum = slot - percentage
		counter++
	}

	for i := 0; i < len(percentages); i++ {
		totalNodesPerKey := int64(seed.NumberOfScenarios) * percentages[i] / 100
		for j := int64(0); j < totalNodesPerKey; j++ {
			nodeName := keys[i] + "-" + core.RandomString(6)
			resultMap[nodeName] = nodeMap[keys[i]]
		}

	}

	return resultMap, nil

}

func GetInstructionScenario(rootNodeName string) models2.ScenarioNode {
	node := models2.ScenarioNode{}
	node.Comment = fmt.Sprintf("**READ CAREFULLY** To create your scenario run plan, assign an ID to each scenario definition (or keep the existing randomly assigned ones if preferred). "+
		"Define dependencies between scenarios using the `depends_on` field, ensuring there are no cycles (including transitive ones) "+
		"or self-references.Nodes not referenced will not be executed, Nodes without dependencies will run first, "+
		"while nodes that share the same parent will execute in parallel. [CURRENT ROOT SCENARIO IS `%s`]", rootNodeName)
	return node
}
