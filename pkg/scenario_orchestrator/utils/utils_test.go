package utils

import (
	"fmt"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"regexp"
	"runtime"
	"testing"
)

func TestFlatten(t *testing.T) {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)

	notExistingKubeconfig := "tests/data/not_existing"
	flatKubeconfigPath, err := PrepareKubeconfig(&notExistingKubeconfig, conf)
	assert.Nil(t, err)
	assert.Nil(t, flatKubeconfigPath)

	wrongKubeconfig := "tests/data/not_existing"
	wrongKubeconfigPath, err := PrepareKubeconfig(&wrongKubeconfig, conf)
	assert.Nil(t, err)
	assert.Nil(t, wrongKubeconfigPath)

	kubeconfigPath := "../../../tests/data/kubeconfig"
	flatKubeconfigPath, err = PrepareKubeconfig(&kubeconfigPath, conf)
	assert.Nil(t, err)
	assert.NotNil(t, flatKubeconfigPath)

	kubeconfig, err := clientcmd.LoadFromFile(*flatKubeconfigPath)
	assert.Nil(t, err)
	assert.NotNil(t, kubeconfig)

	assert.NotEmpty(t, kubeconfig.Clusters)
	ca, err := os.ReadFile("../../../tests/data/test_ca")
	assert.Nil(t, err)
	cert, err := os.ReadFile("../../../tests/data/test_cert")
	assert.Nil(t, err)
	key, err := os.ReadFile("../../../tests/data/test_key")
	assert.Nil(t, err)

	for _, v := range kubeconfig.Clusters {
		assert.NotEmpty(t, v.CertificateAuthorityData)
		assert.Equal(t, ca, v.CertificateAuthorityData)
	}

	for _, auth := range kubeconfig.AuthInfos {
		assert.NotEmpty(t, auth.ClientCertificateData)
		assert.Equal(t, cert, auth.ClientCertificateData)
		assert.NotEmpty(t, auth.ClientKeyData)
		assert.Equal(t, key, auth.ClientKeyData)
	}

}

func TestCleanKubeconfigFiles(t *testing.T) {
	cwd, err := os.Getwd()
	assert.Nil(t, err)
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	kubeconfigPath := "../../../tests/data/kubeconfig"
	flatKubeconfigPath, err := PrepareKubeconfig(&kubeconfigPath, conf)
	assert.Nil(t, err)
	assert.NotNil(t, flatKubeconfigPath)
	root := os.DirFS(cwd)
	mdFiles, err := fs.Glob(root, "krknctl-kubeconfig-*")
	assert.Nil(t, err)
	assert.Greater(t, len(mdFiles), 0)
	num, err := CleanKubeconfigFiles(conf)
	assert.Nil(t, err)
	assert.Equal(t, *num, len(mdFiles))

	mdFiles, err = fs.Glob(root, "krkctl-kubeconifig-*")
	assert.Nil(t, err)
	assert.Equal(t, len(mdFiles), 0)
}

func TestCleanLogFiles(t *testing.T) {
	cwd, err := os.Getwd()
	assert.Nil(t, err)
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	nodename1 := "dummy-random"
	containername1 := GenerateContainerName(conf, "dummy-scenario", &nodename1)
	containername2 := GenerateContainerName(conf, "dummy-scenario", nil)
	filename1 := fmt.Sprintf("%s.log", containername1)
	filename2 := fmt.Sprintf("%s.log", containername2)
	// this one will be kept
	filename3 := "random-log.log"

	file, err := os.Create(filename1)
	assert.Nil(t, err)
	file.Close()
	file, err = os.Create(filename2)
	assert.Nil(t, err)
	file.Close()
	file, err = os.Create(filename3)
	assert.Nil(t, err)
	file.Close()

	root := os.DirFS(cwd)
	mdFiles, err := fs.Glob(root, "*.log")
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(mdFiles), 3)
	num, err := CleanLogFiles(conf)
	assert.Nil(t, err)
	assert.Equal(t, *num, len(mdFiles)-1)

}

func TestGetSocketByContainerEnvironment(t *testing.T) {
	conf, err := krknctlconfig.LoadConfig()
	uid := 1337
	root := 0
	assert.Nil(t, err)
	if runtime.GOOS == "linux" {

		// not testing nil because might be root or user depending on the test environment
		socket, err := GetSocketByContainerEnvironment(models.Podman, conf, &uid)
		assert.Nil(t, err)
		assert.Equal(t, *socket, fmt.Sprintf(conf.PodmanLinuxSocketTemplate, uid))
		socket, err = GetSocketByContainerEnvironment(models.Podman, conf, &root)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.PodmanSocketRoot)

		socket, err = GetSocketByContainerEnvironment(models.Docker, conf, nil)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.DockerSocketRoot)
		socket, err = GetSocketByContainerEnvironment(models.Docker, conf, &uid)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.DockerSocketRoot)
		socket, err = GetSocketByContainerEnvironment(models.Docker, conf, &root)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.DockerSocketRoot)

		_, err = GetSocketByContainerEnvironment(models.Both, conf, &root)
		assert.NotNil(t, err)

	} else if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		assert.Nil(t, err)

		socket, err := GetSocketByContainerEnvironment(models.Podman, conf, &uid)
		assert.Nil(t, err)
		assert.Equal(t, *socket, fmt.Sprintf(conf.PodmanDarwinSocketTemplate, home))

		socket, err = GetSocketByContainerEnvironment(models.Podman, conf, &root)
		assert.Nil(t, err)
		assert.Equal(t, *socket, fmt.Sprintf(conf.PodmanDarwinSocketTemplate, home))

		socket, err = GetSocketByContainerEnvironment(models.Docker, conf, &uid)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.DockerSocketRoot)

		socket, err = GetSocketByContainerEnvironment(models.Docker, conf, &root)
		assert.Nil(t, err)
		assert.Equal(t, *socket, conf.DockerSocketRoot)

		_, err = GetSocketByContainerEnvironment(models.Both, conf, &root)
		assert.NotNil(t, err)

	} else {
		assert.False(t, true, "😱")
	}
}

func TestDetectContainerRuntime(t *testing.T) {
	// **** WARNING
	// Works only on Github Action where both the platforms are installed
	// or on systems where both the platform are installed
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)

	cont, err := DetectContainerRuntime(conf)
	assert.Nil(t, err)
	assert.Equal(t, *cont, models.Both)
}

func TestGenerateContainerName(t *testing.T) {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	nodename := "dummy-random"
	scenarioname := "dummy-scenario"
	renode, err := regexp.Compile(fmt.Sprintf("%s-%s-[0-9]+", conf.ContainerPrefix, nodename))
	assert.Nil(t, err)
	rescenario, err := regexp.Compile(fmt.Sprintf("%s-%s-[0-9]+", conf.ContainerPrefix, scenarioname))
	assert.Nil(t, err)

	containerscenario := GenerateContainerName(conf, scenarioname, nil)
	assert.True(t, rescenario.MatchString(containerscenario))

	containernode := GenerateContainerName(conf, scenarioname, &nodename)
	assert.True(t, renode.MatchString(containernode))

}

func TestEnvironmentFromString(t *testing.T) {

	defer func() {
		if r := recover(); r == nil {
			assert.Fail(t, "unknown string did not panic")
		}
	}()

	podman := EnvironmentFromString("podman")
	assert.Equal(t, podman, models.Podman)

	docker := EnvironmentFromString("docker")
	assert.Equal(t, docker, models.Docker)

	both := EnvironmentFromString("both")
	assert.Equal(t, both, models.Both)

	EnvironmentFromString("unknown")

}

func TestMaskString(t *testing.T) {
	cleanStr := "str123456"
	maskedStr := MaskString(cleanStr)
	assert.Equal(t, maskedStr, "str******")
	shortStr := "str"
	maskedStr = MaskString(shortStr)
	assert.Equal(t, maskedStr, shortStr)
}
