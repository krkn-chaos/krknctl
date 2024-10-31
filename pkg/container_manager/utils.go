package container_manager

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	images "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/text"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type SigTermChannel struct {
	Message string
	Error   error
}

func encodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func PrepareKubeconfig(kubeconfigPath *string, config config.Config) (*string, error) {
	var configPath string

	if kubeconfigPath == nil || *kubeconfigPath == "" {
		configPath = clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	} else {
		path, err := ExpandHomeFolder(*kubeconfigPath)
		if err != nil {
			return nil, err
		}
		configPath = *path
	}

	kubeconfig, err := clientcmd.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	for clusterName, cluster := range kubeconfig.Clusters {
		if cluster.CertificateAuthorityData == nil && cluster.CertificateAuthority != "" {
			certData, err := os.ReadFile(cluster.CertificateAuthority)
			if err != nil {
				return nil, fmt.Errorf("failed to load cluster certificate  '%s': %v", clusterName, err)
			}
			cluster.CertificateAuthorityData = certData
			cluster.CertificateAuthority = ""
		}
	}

	for authName, auth := range kubeconfig.AuthInfos {
		if auth.ClientCertificateData == nil && auth.ClientCertificate != "" {
			certData, err := os.ReadFile(auth.ClientCertificate)
			if err != nil {
				return nil, fmt.Errorf("failed to load user certificate '%s': %v", authName, err)
			}
			auth.ClientCertificateData = certData
			auth.ClientCertificate = ""
		}

		if auth.ClientKeyData == nil && auth.ClientKey != "" {
			keyData, err := os.ReadFile(auth.ClientKey)
			if err != nil {
				return nil, fmt.Errorf("failed to load user key '%s': %v", authName, err)
			}
			auth.ClientKeyData = keyData
			auth.ClientKey = ""
		}
	}

	flattenedConfig, err := clientcmd.Write(*kubeconfig)
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("%s-%s.%d", config.KubeconfigPrefix, text.RandString(5), time.Now().Unix())
	path := filepath.Join(currentDirectory, filename)
	err = os.WriteFile(path, flattenedConfig, 0777)
	if err != nil {
		return nil, err
	}
	return &path, nil
}

func CleanKubeconfigFiles(config config.Config) (*int, error) {
	regex, err := regexp.Compile(fmt.Sprintf("%s-.*\\.[0-9]+", config.KubeconfigPrefix))
	if err != nil {
		return nil, err
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}
	deletedFiles := 0
	for _, file := range files {
		if regex.MatchString(file.Name()) {
			err := os.Remove(filepath.Join(currentDir, file.Name()))
			if err != nil {
				return nil, err
			}
			deletedFiles++
		}
	}
	return &deletedFiles, nil
}

func ExpandHomeFolder(folder string) (*string, error) {
	if strings.HasPrefix(folder, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		replacedHome := strings.Replace(folder, "~/", "", 1)
		expandedPath := filepath.Join(home, replacedHome)
		return &expandedPath, nil
	}
	return &folder, nil
}

func GetSocketByContainerEnvironment(environment ContainerRuntime, config config.Config, userId *int) (*string, error) {
	switch environment {
	case Docker:
		return &config.DockerSocketRoot, nil
	case Podman:
		if runtime.GOOS == "linux" {
			uid := os.Getuid()
			if userId != nil {
				uid = *userId
			}
			if uid == 0 {
				return &config.PodmanSocketRoot, nil
			}
			socket := fmt.Sprintf(config.PodmanLinuxSocketTemplate, uid)
			return &socket, nil
		} else if runtime.GOOS == "darwin" {
			home, _ := os.UserHomeDir()
			socket := fmt.Sprintf(config.PodmanDarwinSocketTemplate, home)
			return &socket, nil
		}
		return nil, errors.New(fmt.Sprintf("could not determine container container runtime socket for podman"))
	case Both:
		return nil, errors.New(fmt.Sprintf("%s invalid container environment", environment.String()))
	}
	return nil, errors.New("invalid environment value")
}

func DetectContainerRuntime(config config.Config) (ContainerRuntime, error) {
	socketDocker, err := GetSocketByContainerEnvironment(Docker, config, nil)
	socketPodman, err := GetSocketByContainerEnvironment(Podman, config, nil)
	defaultEnv := EnvironmentFromString(config.DefaultContainerPlatform)
	if defaultEnv == Both {
		panic(fmt.Sprintf("wrong default platform ´%s´ check your config.json", defaultEnv.String()))
	}
	dockerRuntime := false
	podmanRuntime := false

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(*socketDocker))
	if err != nil {
		dockerRuntime = false
	}
	_, err = cli.ImageList(context.Background(), images.ListOptions{})
	if err == nil {
		dockerRuntime = true
	}

	_, err = bindings.NewConnection(context.Background(), *socketPodman)
	if err == nil {
		podmanRuntime = true
	}

	if podmanRuntime && dockerRuntime {
		return defaultEnv, nil
	}

	if podmanRuntime && dockerRuntime == false {
		return Podman, nil
	}
	if dockerRuntime && podmanRuntime == false {
		return Docker, nil
	}

	return defaultEnv, errors.New("no supported container runtime found")

}
