package models

import (
	"encoding/json"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/typing"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func getScenarioDetail(t *testing.T) ScenarioDetail {
	data := `
{
   "title":"title",
   "description":"description",
   "fields":[
      {
         "name":"testfield_1",
         "type":"string",
         "description":"test field 1",
         "variable":"TESTFIELD_1"
      },
      {
         "name":"testfield_2",
         "type":"number",
         "description":"test field 2",
         "variable":"TESTFIELD_2"
      },      
      {
         "name":"testfield_3",
         "description":"test field 3",
		 "type":"boolean",
         "variable":"TESTFIELD_3"
      },
      {
         "name":"testfield_4",
         "description":"test field 4",
		 "type":"file",
         "variable":"TESTFIELD_4",
         "mount_path":"/mnt/test"
      }
   ]
}
`
	scenarioDetail := ScenarioDetail{}
	err := json.Unmarshal([]byte(data), &scenarioDetail)
	assert.Nil(t, err)
	return scenarioDetail
}

func TestScenarioDetail_GetFieldByEnvVar(t *testing.T) {
	scenarioDetail := getScenarioDetail(t)
	field1 := scenarioDetail.GetFieldByEnvVar("TESTFIELD_1")
	assert.NotNil(t, field1)
	assert.Equal(t, (*field1).Type, typing.String)
	assert.Equal(t, *((*field1).Description), "test field 1")
	assert.Equal(t, *((*field1).Variable), "TESTFIELD_1")
	field2 := scenarioDetail.GetFieldByEnvVar("TESTFIELD_2")
	assert.NotNil(t, field2)
	assert.Equal(t, (*field2).Type, typing.Number)
	assert.Equal(t, *((*field2).Description), "test field 2")
	assert.Equal(t, *((*field2).Variable), "TESTFIELD_2")
	field3 := scenarioDetail.GetFieldByEnvVar("TESTFIELD_3")
	assert.NotNil(t, field3)
	assert.Equal(t, (*field3).Type, typing.Boolean)
	assert.Equal(t, *((*field3).Description), "test field 3")
	assert.Equal(t, *((*field3).Variable), "TESTFIELD_3")

	nofield := scenarioDetail.GetFieldByName("nofield")
	assert.Nil(t, nofield)

}

func TestScenarioDetail_GetFieldByName(t *testing.T) {
	scenarioDetail := getScenarioDetail(t)
	field1 := scenarioDetail.GetFieldByName("testfield_1")
	assert.NotNil(t, field1)
	assert.Equal(t, (*field1).Type, typing.String)
	assert.Equal(t, *((*field1).Description), "test field 1")
	assert.Equal(t, *((*field1).Variable), "TESTFIELD_1")
	field2 := scenarioDetail.GetFieldByName("testfield_2")
	assert.NotNil(t, field2)
	assert.Equal(t, (*field2).Type, typing.Number)
	assert.Equal(t, *((*field2).Description), "test field 2")
	assert.Equal(t, *((*field2).Variable), "TESTFIELD_2")
	field3 := scenarioDetail.GetFieldByName("testfield_3")
	assert.NotNil(t, field3)
	assert.Equal(t, (*field3).Type, typing.Boolean)
	assert.Equal(t, *((*field3).Description), "test field 3")
	assert.Equal(t, *((*field3).Variable), "TESTFIELD_3")

	nofield := scenarioDetail.GetFieldByName("nofield")
	assert.Nil(t, nofield)

}

func TestScenarioDetail_GetFileFieldByMountPath(t *testing.T) {
	scenarioDetail := getScenarioDetail(t)
	field4 := scenarioDetail.GetFileFieldByMountPath("/mnt/test")
	assert.NotNil(t, field4)
	assert.Equal(t, (*field4).Type, typing.File)
	assert.Equal(t, *((*field4).Description), "test field 4")
	assert.Equal(t, *((*field4).Variable), "TESTFIELD_4")

	nofield := scenarioDetail.GetFieldByName("/mnt/notfound")
	assert.Nil(t, nofield)
}

func TestNewRegistryV2FromEnv(t *testing.T) {
	registry := "testregistry.io"
	username := "username"
	password := "password"
	skipTls := true
	insecure := true
	token := "abcdefghilmnopqrstuvz123456790"
	scenarios := "krkn-chaos/krkn-hub"
	config := getConfig(t)
	err := os.Setenv(config.EnvPrivateRegistry, registry)
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistryUsername, username)
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistryPassword, password)
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistryToken, token)
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistrySkipTls, strconv.FormatBool(skipTls))
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistryInsecure, strconv.FormatBool(insecure))
	assert.Nil(t, err)
	err = os.Setenv(config.EnvPrivateRegistryScenarios, scenarios)
	assert.Nil(t, err)

	registryv2, err := NewRegistryV2FromEnv(config)

	assert.Nil(t, err)
	assert.NotNil(t, registryv2)

	assert.Equal(t, registryv2.RegistryUrl, registry)
	assert.Equal(t, *registryv2.Username, username)
	assert.Equal(t, *registryv2.Password, password)
	assert.Equal(t, registryv2.SkipTls, skipTls)
	assert.Equal(t, registryv2.Insecure, insecure)
	assert.Equal(t, *registryv2.Token, token)
	assert.Equal(t, registryv2.ScenarioRepository, scenarios)

	err = os.Unsetenv(config.EnvPrivateRegistryScenarios)
	assert.Nil(t, err)
	registryv2, err = NewRegistryV2FromEnv(config)
	assert.NotNil(t, err)

	// must be true false
	err = os.Setenv(config.EnvPrivateRegistrySkipTls, "True")
	assert.Nil(t, err)
	registryv2, err = NewRegistryV2FromEnv(config)
	assert.NotNil(t, err)

	err = os.Unsetenv(config.EnvPrivateRegistry)
	assert.Nil(t, err)

	registryv2, err = NewRegistryV2FromEnv(config)
	assert.Nil(t, err)
	assert.Nil(t, registryv2)

}
