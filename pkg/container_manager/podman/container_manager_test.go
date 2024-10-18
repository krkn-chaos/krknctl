package podman

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"testing"
)

func getTestConfig() krknctlconfig.Config {
	return krknctlconfig.Config{
		Version:           "0.0.1",
		QuayProtocol:      "https",
		QuayHost:          "quay.io",
		QuayOrg:           "rh_ee_tsebasti",
		QuayRegistry:      "krknctl",
		QuayRepositoryApi: "api/v1/repository",
	}
}

func TestConnect(t *testing.T) {
	/*	env := map[string]string{
			"CHAOS_DURATION": "10",
			"CORES":          "1",
			"CPU_PERCENTAGE": "60",
			"NAMESPACE":      "default",
		}
		conf := getTestConfig()
		cm := ContainerManager{}
		quayProvider := quay.ScenarioProvider{}
		scenario, err := quayProvider.GetScenarioDetail("cpu-hog", conf.GetQuayRepositoryApiUri())
		assert.Nil(t, err)
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		containerId, err := cm.RunAndStream(conf.GetQuayImageUri()+":"+scenario.Name,
			scenario.Name,
			"unix://run/podman/podman.sock",
			env, false, map[string]string{}, home+"/kubeconfig", scenario.KubeconfigPath)
		assert.Nil(t, err)
		assert.NotNil(t, containerId)*/

}
