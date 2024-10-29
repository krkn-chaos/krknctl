package config

import (
	_ "embed"
	"encoding/json"
)

type Config struct {
	Version                    string `json:"version"`
	QuayProtocol               string `json:"quay_protocol"`
	QuayHost                   string `json:"quay_host"`
	QuayOrg                    string `json:"quay_org"`
	QuayRegistry               string `json:"quay_registry"`
	QuayRepositoryApi          string `json:"quay_repositoryApi"`
	ContainerPrefix            string `json:"container_prefix"`
	KubeconfigPrefix           string `json:"kubeconfig_prefix"`
	PodmanDarwinSocketTemplate string `json:"podman_darwin_socket_template"`
	PodmanLinuxSocketTemplate  string `json:"podman_linux_socket_template"`
	PodmanSocketRoot           string `json:"podman_socket_root_linux"`
	DockerSocketRoot           string `json:"docker_socket_root"`
}

//go:embed config.json
var File []byte

func LoadConfig() (Config, error) {
	var config Config
	err := json.Unmarshal(File, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}

func (c *Config) GetQuayImageUri() string {
	return c.QuayHost + "/" + c.QuayOrg + "/" + c.QuayRegistry
}

func (c *Config) GetQuayRegistryUri() string {
	return c.QuayProtocol + "://" + c.QuayHost + "/" + c.QuayOrg + "/" + c.QuayRegistry
}

func (c *Config) GetQuayRepositoryApiUri() string {

	return c.QuayProtocol + "://" + c.QuayHost + "/" + c.QuayRepositoryApi + "/" + c.QuayOrg + "/" + c.QuayRegistry
}
