package config

import (
	_ "embed"
	"encoding/json"
	"net/url"
)

type Config struct {
	Version                    string `json:"version"`
	QuayHost                   string `json:"quay_host"`
	QuayOrg                    string `json:"quay_org"`
	QuayRegistry               string `json:"quay_registry"`
	QuayRepositoryApi          string `json:"quay_repositoryApi"`
	ContainerPrefix            string `json:"container_prefix"`
	KubeconfigPrefix           string `json:"kubeconfig_prefix"`
	PodmanDarwinSocketTemplate string `json:"podman_darwin_socket_template"`
	PodmanLinuxSocketTemplate  string `json:"podman_linux_socket_template"`
	PodmanSocketRoot           string `json:"podman_socket_root_linux"`
	PodmanRunningState         string `json:"podman_running_state"`
	DockerSocketRoot           string `json:"docker_socket_root"`
	DockerRunningState         string `json:"docker_running_state"`
	DefaultContainerPlatform   string `json:"default_container_platform"`
	MetricsProfilePath         string `json:"metrics_profile_path"`
	AlertsProfilePath          string `json:"alerts_profile_path"`
	KubeconfigPath             string `json:"kubeconfig_path"`
	LabelTitle                 string `json:"label_title"`
	LabelDescription           string `json:"label_description"`
	LabelInputFields           string `json:"label_input_fields"`
	LabelTitleRegex            string `json:"label_title_regex"`
	LabelDescriptionRegex      string `json:"label_description_regex"`
	LabelInputFieldsRegex      string `json:"label_input_fields_regex"`
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

func (c *Config) GetQuayImageUri() (string, error) {
	imageUri, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.QuayRegistry)
	if err != nil {
		return "", err
	}
	return imageUri, nil
}

func (c *Config) GetQuayRepositoryApiUri() (string, error) {
	baseHost := "https://" + c.QuayHost
	repositoryUri, err := url.JoinPath(baseHost, c.QuayRepositoryApi, c.QuayOrg, c.QuayRegistry)
	if err != nil {
		return "", err
	}
	return repositoryUri, nil
}
