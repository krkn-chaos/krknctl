package quay

import (
	jsonparser "encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	providerinterface "github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
	"time"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func getTestConfig(t *testing.T) krknctlconfig.Config {
	conf := getConfig(t)
	conf.QuayScenarioRegistry = "krknctl-test"
	return conf
}

func getWrongConfig(t *testing.T) krknctlconfig.Config {
	conf := getConfig(t)
	conf.QuayScenarioRegistry = "do_not_exist"
	return conf
}

func TestScenarioProvider_GetRegistryImages(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	scenarios, err := provider.GetRegistryImages(nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	assert.Greater(t, len(*scenarios), 0)
	for i := range *scenarios {
		assert.NotEqual(t, (*scenarios)[i].Name, "")
		assert.NotEqual(t, (*scenarios)[i].Digest, "")
		assert.NotEqual(t, (*scenarios)[i].Size, 0)
		assert.NotEqual(t, (*scenarios)[i].LastModified, time.Time{})
	}

	wrongConfig := getWrongConfig(t)
	wrongProvider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: wrongConfig,
			Cache:  cache.NewCache(),
		},
	}
	_, err = wrongProvider.GetRegistryImages(nil)
	assert.Error(t, err)

}

func TestScenarioProvider_GetRegistryImages_ResolvesMissingManifestListSize(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{providerinterface.BaseScenarioProvider{
		Config: config,
		Cache:  cache.NewCache(),
	}}
	dataSource := "https://quay.example/api/v1/repository/krkn-hub-multiarch"
	tagURL := dataSource + "/tag"
	manifestURL := dataSource + "/manifest/sha256:index"
	selectedManifestURL := dataSource + "/manifest/sha256:linux"
	provider.Cache.Set(tagURL, []byte(`{"tags":[{"name":"multiarch","manifest_digest":"sha256:index","last_modified":"Mon, 02 Jan 2023 12:00:00 +0000"}]}`))
	provider.Cache.Set(manifestURL, []byte(`{"is_manifest_list":true,"manifest_data":"{\"manifests\":[{\"digest\":\"sha256:linux\",\"size\":12345}]}"}`))
	provider.Cache.Set(selectedManifestURL, []byte(`{"layers":[{"compressed_size":5242880,"created_datetime":"Mon, 02 Jan 2023 12:00:00 +0000"}]}`))

	tags, err := provider.getRegistryImages(dataSource)
	assert.NoError(t, err)
	if assert.Len(t, *tags, 1) {
		require.NotNil(t, (*tags)[0].Size)
		assert.Equal(t, int64(5242880), *(*tags)[0].Size)
	}
}

func TestQuayScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getTestConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	scenario, err := provider.GetScenarioDetail("cpu-hog", nil)

	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Equal(t, len(scenario.Fields), 5)
	assert.NotNil(t, scenario.Size, "scenario detail must preserve the authoritative Quay image size")

	scenario, err = provider.GetScenarioDetail("cpu-memory-notitle", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.title LABEL not found in tag: cpu-memory-notitle"))
	assert.True(t, errors.Is(err, providerinterface.ErrLabelNotFound))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-nodescription", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.description LABEL not found in tag: cpu-memory-nodescription"))
	assert.True(t, errors.Is(err, providerinterface.ErrLabelNotFound))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("cpu-memory-noinput", nil)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "krknctl.input_fields LABEL not found in tag: cpu-memory-noinput"))
	assert.True(t, errors.Is(err, providerinterface.ErrLabelNotFound))
	assert.Nil(t, scenario)

	scenario, err = provider.GetScenarioDetail("not-found", nil)
	assert.Nil(t, err)
	assert.Nil(t, scenario)

}

func TestQuayScenarioProvider_ScaffoldScenarios(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	scenarios, err := provider.GetRegistryImages(nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	scenarioNames := []string{"node-cpu-hog", "node-memory-hog", "dummy-scenario"}

	json, err := provider.ScaffoldScenarios(scenarioNames, false, nil, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	fmt.Println(os.Getwd())
	seed := providerinterface.ScaffoldSeed{
		Path:              "../../../tests/data/scaffold-seed.json",
		NumberOfScenarios: 1000,
	}

	json, err = provider.ScaffoldScenarios([]string{}, false, nil, false, &seed)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	var scenariodetails map[string]models.ScenarioDetail
	err = jsonparser.Unmarshal([]byte(*json), &scenariodetails)
	assert.Nil(t, err)
	assert.Equal(t, len(scenariodetails), seed.NumberOfScenarios)

	json, err = provider.ScaffoldScenarios(scenarioNames, false, nil, true, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	var parsedScenarios map[string]map[string]interface{}
	err = jsonparser.Unmarshal([]byte(*json), &parsedScenarios)
	assert.Nil(t, err)
	for el := range parsedScenarios {
		assert.NotEqual(t, el, "comment")
		_, ok := parsedScenarios[el]["depends_on"]
		assert.False(t, ok)
	}

	json, err = provider.ScaffoldScenarios(scenarioNames, true, nil, false, nil)
	if err != nil {
		if strings.Contains(err.Error(), "american-english dictionary") || strings.Contains(err.Error(), "babble") {
			t.Skipf("dictionary dependency missing: %v", err)
		}
	}
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = provider.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, false, nil, false, nil)
	assert.Nil(t, json)
	assert.NotNil(t, err)

}

func TestQuayScenarioProvider_GetGlobalEnvironment(t *testing.T) {
	config := getConfig(t)
	provider := ScenarioProvider{
		providerinterface.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	config.QuayBaseImageRegistry = "krknctl-test"
	baseImageScenario, err := provider.GetGlobalEnvironment(nil, "node-cpu-hog")
	assert.Nil(t, err)
	assert.NotNil(t, baseImageScenario)
	assert.Greater(t, len(baseImageScenario.Fields), 0)
}
