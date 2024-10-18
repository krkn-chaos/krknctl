package container_manager

import (
	"encoding/base64"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/text"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"time"
)

func encodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func PrepareKubeconfig(kubeconfigPath *string) (*string, error) {
	var configPath string

	if kubeconfigPath == nil || *kubeconfigPath == "" {
		configPath = clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	} else {
		configPath = *kubeconfigPath
	}

	config, err := clientcmd.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	for clusterName, cluster := range config.Clusters {
		if cluster.CertificateAuthorityData == nil && cluster.CertificateAuthority != "" {
			certData, err := os.ReadFile(cluster.CertificateAuthority)
			if err != nil {
				return nil, fmt.Errorf("failed to load cluster certificate  '%s': %v", clusterName, err)
			}
			cluster.CertificateAuthorityData = certData
			cluster.CertificateAuthority = ""
		}
	}

	for authName, auth := range config.AuthInfos {
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

	flattenedConfig, err := clientcmd.Write(*config)
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf(".krknctl-kubeconfig-%s.%d", text.RandString(5), time.Now().Unix())
	path := filepath.Join(currentDirectory, filename)
	err = os.WriteFile(path, flattenedConfig, 0777)
	if err != nil {
		return nil, err
	}
	return &path, nil
}

func DetectContainerManager() Environment {
	return Podman
}
