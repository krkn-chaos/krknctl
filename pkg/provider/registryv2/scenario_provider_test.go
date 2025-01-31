package registryv2

import (
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func TestScenarioProvider_GetRegistryImages_PublicRegistry(t *testing.T) {
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
		},
	}
	r := models.RegistryV2{
		RegistryUrl:         "quay.io",
		ScenarioRepository:  "krkn-chaos/krkn-hub",
		BaseImageRepository: "krkn-chaos/krkn",
		UseTLS:              true,
	}
	tags, err := p.GetRegistryImages(&r)
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)
}

/*
To authenticate with quay via jwt token:
curl -X GET \
  --user '<username>:password' \
  "https://quay.io/v2/auth?service=quay.io&scope=repository:rh_ee_tsebasti/krkn-hub-private:pull,push" \
  -k | jq -r '.token'
*/

func TestScenarioProvider_GetRegistryImages_jwt(t *testing.T) {
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	pr := models.RegistryV2{
		RegistryUrl:         "quay.io",
		ScenarioRepository:  "rh_ee_tsebasti/krkn-hub-private",
		BaseImageRepository: "rh_ee_tsebasti/krkn-private",
		Token:               &quayToken,
		UseTLS:              true,
	}

	tags, err := p.GetRegistryImages(&pr)
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)

	quayToken = "wrong"
	pr = models.RegistryV2{
		RegistryUrl:         "quay.io",
		ScenarioRepository:  "rh_ee_tsebasti/krkn-hub-private",
		BaseImageRepository: "rh_ee_tsebasti/krkn-private",
		Token:               &quayToken,
		UseTLS:              true,
	}

	_, err = p.GetRegistryImages(&pr)
	assert.NotNil(t, err)
}

/*
To run an example registry with basic auth:

mkdir auth

docker run \
  --entrypoint htpasswd \
  httpd:2 -Bbn testuser testpassword > auth/htpasswd

docker run -d \
  -p 5001:5000 \
  --restart=always \
  --name registry \
  -v "$(pwd)"/auth:/auth \
  -e "REGISTRY_AUTH=htpasswd" \
  -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
  -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
  registry:2
*/

func TestScenarioProvider_GetRegistryImages_Htpasswd(t *testing.T) {
	basicAuthUsername := "testuser"
	basicAuthPassword := "testpassword"

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
		},
	}
	pr := models.RegistryV2{
		RegistryUrl:         "localhost:5001",
		ScenarioRepository:  "krkn-chaos/krkn-hub",
		BaseImageRepository: "krkn-chaos/krkn",
		Username:            &basicAuthUsername,
		Password:            &basicAuthPassword,
		UseTLS:              false,
	}

	tags, err := p.GetRegistryImages(&pr)
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)

	basicAuthUsername = "wrong"
	basicAuthPassword = "wrong"
	pr = models.RegistryV2{
		RegistryUrl:         "localhost:5001",
		ScenarioRepository:  "krkn-chaos/krkn-hub",
		BaseImageRepository: "krkn-chaos/krkn",
		Username:            &basicAuthUsername,
		Password:            &basicAuthPassword,
		UseTLS:              false,
	}
	_, err = p.GetRegistryImages(&pr)
	assert.NotNil(t, err)

}

func TestScenarioProvider_GetScenarioDetail(t *testing.T) {
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	pr := models.RegistryV2{
		RegistryUrl:         "quay.io",
		ScenarioRepository:  "rh_ee_tsebasti/krkn-hub-private",
		BaseImageRepository: "rh_ee_tsebasti/krkn-private",
		Token:               &quayToken,
		UseTLS:              true,
	}

	res, err := p.GetScenarioDetail("dummy-scenario", &pr)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.Name, "dummy-scenario")
	assert.Equal(t, res.Title, "Dummy Scenario")
	assert.Equal(t, len(res.Fields), 2)
	assert.NotNil(t, res.ScenarioTag.Name)
	assert.Nil(t, res.ScenarioTag.Size)
	assert.Nil(t, res.ScenarioTag.LastModified)
	assert.Nil(t, res.ScenarioTag.Digest)

}
