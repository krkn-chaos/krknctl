// Package utils provides utility functions for the container runtime environment
package utils

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	images "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/krkn-chaos/krknctl/pkg/config"
	orchestatormodels "github.com/krkn-chaos/krknctl/pkg/scenario_orchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/text"
	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"
	"io/fs"
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

func PrepareKubeconfig(kubeconfigPath *string, config config.Config) (*string, error) {
	var configPath string
	var configFolder string
	if kubeconfigPath == nil || *kubeconfigPath == "" {
		configPath = clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	} else {
		path, err := commonutils.ExpandFolder(*kubeconfigPath, nil)
		if err != nil {
			return nil, err
		}
		configPath = *path
	}

	configFolder = filepath.Dir(configPath)

	kubeconfig, err := clientcmd.LoadFromFile(configPath)
	if err != nil {
		var fserr *fs.PathError
		if errors.As(err, &fserr) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	for clusterName, cluster := range kubeconfig.Clusters {
		if cluster.CertificateAuthorityData == nil && cluster.CertificateAuthority != "" {
			path, err := commonutils.ExpandFolder(cluster.CertificateAuthority, &configFolder)
			if err != nil {
				return nil, err
			}

			certData, err := os.ReadFile(*path)
			if err != nil {
				return nil, fmt.Errorf("failed to load cluster certificate  '%s': %v", clusterName, err)
			}
			cluster.CertificateAuthorityData = certData
			cluster.CertificateAuthority = ""
		}
	}

	for authName, auth := range kubeconfig.AuthInfos {
		if auth.ClientCertificateData == nil && auth.ClientCertificate != "" {
			path, err := commonutils.ExpandFolder(auth.ClientCertificate, &configFolder)
			if err != nil {
				return nil, err
			}
			certData, err := os.ReadFile(*path)
			if err != nil {
				return nil, fmt.Errorf("failed to load user certificate '%s': %v", authName, err)
			}
			auth.ClientCertificateData = certData
			auth.ClientCertificate = ""
		}

		if auth.ClientKeyData == nil && auth.ClientKey != "" {
			path, err := commonutils.ExpandFolder(auth.ClientKey, &configFolder)
			if err != nil {
				return nil, err
			}
			keyData, err := os.ReadFile(*path)
			if err != nil {
				return nil, fmt.Errorf("failed to load user key '%s': %v", authName, err)
			}
			auth.ClientKeyData = keyData
			auth.ClientKey = ""
		}
	}

	flattenedConfig, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, err
	}
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("%s-%s-%d", config.KubeconfigPrefix, text.RandString(5), time.Now().Unix())
	path := filepath.Join(currentDirectory, filename)
	err = os.WriteFile(path, flattenedConfig, 0600)
	if err != nil {
		return nil, err
	}
	return &path, nil
}

func CleanKubeconfigFiles(config config.Config) (*int, error) {
	regex, err := regexp.Compile(fmt.Sprintf("^%s-.*-[0-9]+$", config.KubeconfigPrefix))
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

func CleanLogFiles(config config.Config) (*int, error) {
	regex, err := regexp.Compile(fmt.Sprintf("%s-.*\\-[0-9]+\\.log", config.ContainerPrefix))
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

func GetSocketByContainerEnvironment(environment orchestatormodels.ContainerRuntime, config config.Config, userID *int) (*string, error) {
	switch environment {
	case orchestatormodels.Docker:
		return &config.DockerSocketRoot, nil
	case orchestatormodels.Podman:
		if runtime.GOOS == "linux" {
			uid := os.Getuid()
			if userID != nil {
				uid = *userID
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
		return nil, fmt.Errorf("could not determine container container runtime socket for podman")
	case orchestatormodels.Both:
		return nil, fmt.Errorf("%s invalid container environment", environment.String())
	}
	return nil, errors.New("invalid environment value")
}

func DetectContainerRuntime(config config.Config) (*orchestatormodels.ContainerRuntime, error) {
	socketDocker, err := GetSocketByContainerEnvironment(orchestatormodels.Docker, config, nil)
	if err != nil {
		return nil, err
	}
	socketPodman, err := GetSocketByContainerEnvironment(orchestatormodels.Podman, config, nil)
	if err != nil {
		return nil, err
	}
	_docker := orchestatormodels.Docker
	_podman := orchestatormodels.Podman
	_both := orchestatormodels.Both
	dockerRuntime := false
	podmanRuntime := false

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(*socketDocker))
	if cli == nil {
		return nil, fmt.Errorf("could not determine docker runtime")
	}
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
		return &_both, nil
	}

	if podmanRuntime {
		return &_podman, nil
	}
	if dockerRuntime {
		return &_docker, nil
	}

	return nil, fmt.Errorf("could not determine docker runtime")

}

func GenerateContainerName(config config.Config, scenarioName string, graphNodeName *string) string {
	// sleep is needed to avoid the same timestamp to two different containers
	// and break all the logic related to the container name timestamp

	time.Sleep(1 * time.Nanosecond)
	if graphNodeName != nil {
		return fmt.Sprintf("%s-%s-%d", config.ContainerPrefix, *graphNodeName, time.Now().Nanosecond())
	}
	return fmt.Sprintf("%s-%s-%d", config.ContainerPrefix, scenarioName, time.Now().Nanosecond())
}

func EnvironmentFromString(s string) orchestatormodels.ContainerRuntime {
	switch s {
	case "Podman":
		return orchestatormodels.Podman
	case "Docker":
		return orchestatormodels.Docker
	case "Both":
		return orchestatormodels.Both
	default:
		panic(fmt.Sprintf("unknown container environment: %q", s))
	}
}

func MaskString(s string) string {
	if len(s) < 3 {
		return s
	}
	return s[:3] + strings.Repeat("*", len(s)-3)
}
