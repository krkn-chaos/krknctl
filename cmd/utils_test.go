package cmd

import (
	"encoding/json"
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	providerModels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func TestNewSpinnerWithSuffix(t *testing.T) {
	spinner := NewSpinnerWithSuffix("test")
	assert.NotNil(t, spinner)
	assert.Equal(t, "test", spinner.Suffix)
}

func TestGetProvider(t *testing.T) {
	config := getConfig(t)
	providerFactory := providerfactory.NewProviderFactory(&config)
	quayProvider := providerFactory.NewInstance(provider.Quay)
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.NotNil(t, scenario.Size)
	assert.Greater(t, *scenario.Size, int64(0))

	scenario, err = quayProvider.GetScenarioDetail("do-not-exist", nil)
	assert.Nil(t, err)
	assert.Nil(t, scenario)

	basicAuthUsername := "testuser"
	basicAuthPassword := "testpassword"

	pr := providerModels.RegistryV2{
		RegistryUrl:        "localhost:5001",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		Username:           &basicAuthUsername,
		Password:           &basicAuthPassword,
		Insecure:           true,
	}

	privateProvider := providerFactory.NewInstance(provider.Private)
	scenario, err = privateProvider.GetScenarioDetail("dummy-scenario", &pr)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.Nil(t, scenario.Size)

}

func TestFetchScenarios(t *testing.T) {
	config := getConfig(t)
	providerFactory := providerfactory.NewProviderFactory(&config)
	quayProvider := providerFactory.NewInstance(provider.Quay)
	scenarios, err := FetchScenarios(quayProvider, nil)
	assert.Nil(t, err)
	assert.Greater(t, len(*scenarios), 0)

	basicAuthUsername := "testuser"
	basicAuthPassword := "testpassword"

	pr := providerModels.RegistryV2{
		RegistryUrl:        "localhost:5001",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		Username:           &basicAuthUsername,
		Password:           &basicAuthPassword,
		Insecure:           true,
	}

	privateProvider := providerFactory.NewInstance(provider.Private)
	scenarios, err = FetchScenarios(privateProvider, &pr)
	assert.Nil(t, err)
	assert.Greater(t, len(*scenarios), 0)

}

func TestCheckFileExists(t *testing.T) {
	_, err := os.Create("test.txt")
	assert.Nil(t, err)
	assert.True(t, CheckFileExists("test.txt"))
	assert.False(t, CheckFileExists("donotexist.txt"))

}

func TestParseFlags(t *testing.T) {
	config := getConfig(t)
	providerFactory := providerfactory.NewProviderFactory(&config)
	quayProvider := providerFactory.NewInstance(provider.Quay)
	scenario, err := quayProvider.GetScenarioDetail("dummy-scenario", nil)
	assert.Nil(t, err)
	assert.NotNil(t, scenario)
	assert.NotNil(t, scenario.Size)
	assert.Greater(t, *scenario.Size, int64(0))
	fileName := "test.txt"
	_, err = os.Create(fileName)
	assert.Nil(t, err)
	scenarioFields := make(map[string]*string)
	scenarioFlags := pflag.NewFlagSet("scenario", pflag.ExitOnError)
	for _, field := range scenario.Fields {
		var defaultValue = ""
		if field.Default != nil {
			defaultValue = *field.Default
		}
		scenarioFields[*field.Name] = scenarioFlags.String(*field.Name, defaultValue, *field.Description)
	}

	// happy path all values set
	args := []string{"dummy-scenario", "--duration", "20", "--exit", "1", "--mounted_file", fileName, "--secret_string", "test"}
	environment, volume, err := ParseFlags(scenario, args, scenarioFields, false)
	assert.Nil(t, err)
	assert.NotNil(t, environment)
	assert.NotNil(t, volume)

	env := *environment
	vol := *volume
	duration := env["END"]
	exit := env["EXIT_STATUS"]
	secretString := env["SECRET_STRING"]
	mountedFile := env["MOUNTED_FILE"]

	assert.NotNil(t, duration)
	assert.Equal(t, "20", duration.value)
	assert.Equal(t, "1", exit.value)
	assert.Equal(t, "test", secretString.value)
	assert.True(t, secretString.secret)

	cwd, err := os.Getwd()
	assert.Nil(t, err)
	assert.Contains(t, *volume, filepath.Join(cwd, fileName))
	assert.Equal(t, mountedFile.value, vol[filepath.Join(cwd, fileName)])

	// default values
	args = []string{}
	environment, volume, err = ParseFlags(scenario, args, scenarioFields, false)
	assert.Nil(t, err)

	env = *environment
	vol = *volume
	duration = env["END"]
	exit = env["EXIT_STATUS"]
	secretString = env["SECRET_STRING"]

	assert.Equal(t, "10", duration.value)
	assert.Equal(t, "0", exit.value)
	assert.Equal(t, "", secretString.value)

	assert.Equal(t, len(*volume), 0)

	// bad value
	args = []string{"dummy-scenario", "--duration", "string"}
	environment, volume, err = ParseFlags(scenario, args, scenarioFields, false)
	assert.NotNil(t, err)

}

func TestParsePrivateFlags(t *testing.T) {
	cmd := NewListCommand()
	assert.NotNil(t, cmd)
	registry, err := parsePrivateRepoArgs(cmd, nil)
	assert.Nil(t, err)
	assert.Nil(t, registry)
	username := "username"
	password := "password"
	skipTls := true
	insecure := true
	token := "token"
	registryScenarios := "repository/registry"
	registryHost := "registry.example.com"
	cmd.Flags().StringVar(&username, "private-registry-username", username, "")
	cmd.Flags().StringVar(&password, "private-registry-password", password, "")
	cmd.Flags().BoolVar(&skipTls, "private-registry-skip-tls", skipTls, "")
	cmd.Flags().Lookup("private-registry-skip-tls").Changed = true
	cmd.Flags().BoolVar(&insecure, "private-registry-insecure", insecure, "")
	cmd.Flags().Lookup("private-registry-insecure").Changed = true
	cmd.Flags().StringVar(&token, "private-registry-token", token, "")
	cmd.Flags().StringVar(&registryScenarios, "private-registry-scenarios", "", "")
	registry, err = parsePrivateRepoArgs(cmd, nil)
	assert.Nil(t, err)
	assert.Nil(t, registry)
	cmd.Flags().StringVar(&registryHost, "private-registry", registryHost, "")
	cmd.Flags().Lookup("private-registry").Changed = true
	registry, err = parsePrivateRepoArgs(cmd, nil)
	assert.NotNil(t, err)
	cmd.Flags().Set("private-registry-scenarios", "registry.example.com")
	registry, err = parsePrivateRepoArgs(cmd, nil)
	assert.Nil(t, err)
	assert.NotNil(t, registry)

	assert.Equal(t, *registry.Username, username)
	assert.Equal(t, *registry.Password, password)
	assert.True(t, registry.Insecure)
	assert.True(t, registry.SkipTls)
	assert.Equal(t, *registry.Token, "token")

	cmd.DisableFlagParsing = true

	registry, err = parsePrivateRepoArgs(cmd, nil)
	assert.NotNil(t, err)

	var args []string

	registry, err = parsePrivateRepoArgs(cmd, &args)
	assert.Nil(t, err)
	assert.Nil(t, registry)

	args = []string{"--private-registry-username", username,
		"--private-registry-password", password,
		"--private-registry-skip-tls", "true",
		"--private-registry-insecure", "true",
		"--private-registry-token", token,
		"--private-registry-scenario", registryScenarios,
	}

	registry, err = parsePrivateRepoArgs(cmd, &args)
	assert.Nil(t, err)
	assert.Nil(t, registry)

	args = []string{"--private-registry-username", username,
		"--private-registry-password", password,
		"--private-registry-skip-tls", "true",
		"--private-registry-insecure", "true",
		"--private-registry-token", token,
		"--private-registry", registryHost,
	}

	registry, err = parsePrivateRepoArgs(cmd, &args)
	assert.NotNil(t, err)

	args = []string{
		"--private-registry-username", username,
		"--private-registry-password", password,
		"--private-registry-skip-tls",
		"--private-registry-insecure",
		"--private-registry-token", token,
		"--private-registry", registryHost,
		"--private-registry-scenarios", registryScenarios,
	}

	registry, err = parsePrivateRepoArgs(cmd, &args)
	assert.Nil(t, err)
	assert.NotNil(t, registry)
	assert.Equal(t, *registry.Username, username)
	assert.Equal(t, *registry.Password, password)
	assert.True(t, registry.Insecure)
	assert.True(t, registry.SkipTls)
	assert.Equal(t, *registry.Token, token)
}

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

func TestGetLatest(t *testing.T) {
	config := getConfig(t)
	latest, err := GetLatest(config)
	assert.Nil(t, err)
	assert.NotNil(t, latest)
	config.GithubLatestReleaseAPI = "https://httpstat.us/200?sleep=3000"
	latest, err = GetLatest(config)
	assert.Nil(t, err)
	assert.Nil(t, err)
	config.GithubLatestReleaseAPI = "https://httpstat.us/404"
	latest, err = GetLatest(config)
	assert.NotNil(t, err)
	assert.Nil(t, latest)
}

func TestIsDeprecated(t *testing.T) {
	config := getConfig(t)
	config.Version = "v0.9.1-beta"
	deprecated, err := IsDeprecated(config)
	assert.Nil(t, err)
	assert.NotNil(t, deprecated)
	assert.False(t, *deprecated)
	config.Version = "donotexist"
	deprecated, err = IsDeprecated(config)
	assert.NotNil(t, err)
	assert.Nil(t, deprecated)
	config.Version = "v0.1.2-alpha"
	deprecated, err = IsDeprecated(config)
	assert.Nil(t, err)
	assert.True(t, *deprecated)
}
