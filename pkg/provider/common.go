package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	models2 "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/tjarratt/babble"
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

func ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, config config.Config, p ScenarioDataProvider) (*string, error) {
	var scenarioDetails []models.ScenarioDetail

	// handles babble panic when american word dictionary is not installed
	defer func() {
		if err := recover(); err != nil {
			_, err = color.New(color.FgRed, color.Underline).Println("impossible to locate american words dictionary " +
				"please refer to the documentation on how to install this dependency https://github.com/krkn-chaos/krknctl#Requirements")
		}
	}()
	babbler := babble.NewBabbler()
	babbler.Count = 1
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

	scenarioNodes["_comment"] = GetInstructionScenario(indexes[0])
	for i, scenarioDetail := range scenarioDetails {
		indexes = append(indexes, strings.ToLower(babbler.Babble()))

		scenarioNode := models2.ScenarioNode{}
		if i > 0 {
			scenarioNode.Parent = &indexes[i-1]
		} else {
			scenarioNode.Comment = "I'm the root Node!"
		}

		var imageUri = ""
		if registry == nil {
			uri, err := config.GetQuayImageUri()
			if err != nil {
				return nil, err
			}
			imageUri = uri
		} else {
			imageUri = registry.GetPrivateRegistryUri()
		}
		scenarioNode.Image = imageUri + ":" + scenarioDetail.Name
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
					if detail.Required == true {
						required = "(required)"
					}
					scenarioNode.Env[*detail.Variable] = fmt.Sprintf("<%s%s>", *detail.Description, required)
				}

			}

		}

		if includeGlobalEnv == true {
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

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(scenarioNodes)
	if err != nil {
		return nil, err
	}
	jsonBuf := buf.String()
	return &jsonBuf, nil
}

func GetInstructionScenario(rootNodeName string) models2.ScenarioNode {
	node := models2.ScenarioNode{}
	node.Comment = fmt.Sprintf("**READ CAREFULLY** To create your scenario run plan, assign an ID to each scenario definition (or keep the existing randomly assigned ones if preferred). "+
		"Define dependencies between scenarios using the `depends_on` field, ensuring there are no cycles (including transitive ones) "+
		"or self-references.Nodes not referenced will not be executed, Nodes without dependencies will run first, "+
		"while nodes that share the same parent will execute in parallel. [CURRENT ROOT SCENARIO IS `%s`]", rootNodeName)
	return node
}
