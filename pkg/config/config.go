package config

import (
	_ "embed"
	"encoding/json"
	"net/url"
)

type Config struct {
	Version                     string `json:"version"`
	QuayHost                    string `json:"quay_host"`
	QuayOrg                     string `json:"quay_org"`
	QuayScenarioRegistry        string `json:"quay_scenario_registry"`
	QuayBaseImageRegistry       string `json:"quay_base_image_registry"`
	QuayBaseImageTag            string `json:"quay_base_image_tag"`
	QuayRepositoryApi           string `json:"quay_repository_api"`
	PrivateRegistryBaseImageTag string `json:"private_registry_base_image_tag"`
	ContainerPrefix             string `json:"container_prefix"`
	KubeconfigPrefix            string `json:"kubeconfig_prefix"`
	PodmanDarwinSocketTemplate  string `json:"podman_darwin_socket_template"`
	PodmanLinuxSocketTemplate   string `json:"podman_linux_socket_template"`
	PodmanSocketRoot            string `json:"podman_socket_root_linux"`
	PodmanRunningState          string `json:"podman_running_state"`
	DockerSocketRoot            string `json:"docker_socket_root"`
	DockerRunningState          string `json:"docker_running_state"`
	DefaultContainerPlatform    string `json:"default_container_platform"`
	MetricsProfilePath          string `json:"metrics_profile_path"`
	AlertsProfilePath           string `json:"alerts_profile_path"`
	KubeconfigPath              string `json:"kubeconfig_path"`
	LabelTitle                  string `json:"label_title"`
	LabelDescription            string `json:"label_description"`
	LabelInputFields            string `json:"label_input_fields"`
	LabelTitleGlobal            string `json:"label_title_global"`
	LabelDescriptionGlobal      string `json:"label_description_global"`
	LabelInputFieldsGlobal      string `json:"label_input_fields_global"`
	LabelTitleRegex             string `json:"label_title_regex"`
	LabelDescriptionRegex       string `json:"label_description_regex"`
	LabelInputFieldsRegex       string `json:"label_input_fields_regex"`
	LabelTitleRegexGlobal       string `json:"label_title_regex_global"`
	LabelDescriptionRegexGlobal string `json:"label_description_regex_global"`
	LabelInputFieldsRegexGlobal string `json:"label_input_fields_regex_global"`
	EnvPrivateRegistry          string `json:"env_private_registry"`
	EnvPrivateRegistryUsername  string `json:"env_private_registry_username"`
	EnvPrivateRegistryPassword  string `json:"env_private_registry_password"`
	EnvPrivateRegistrySkipTls   string `json:"env_private_registry_skip_tls"`
	EnvPrivateRegistryToken     string `json:"env_private_registry_token"`
	EnvPrivateRegistryScenarios string `json:"env_private_registry_scenarios"`
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
	imageUri, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.QuayScenarioRegistry)
	if err != nil {
		return "", err
	}
	return imageUri, nil
}

func (c *Config) GetQuayScenarioRepositoryApiUri() (string, error) {
	baseHost := "https://" + c.QuayHost
	repositoryUri, err := url.JoinPath(baseHost, c.QuayRepositoryApi, c.QuayOrg, c.QuayScenarioRegistry)
	if err != nil {
		return "", err
	}
	return repositoryUri, nil
}

func (c *Config) GetQuayBaseImageRepositoryApiUri() (string, error) {
	baseHost := "https://" + c.QuayHost
	repositoryUri, err := url.JoinPath(baseHost, c.QuayRepositoryApi, c.QuayOrg, c.QuayBaseImageRegistry)
	if err != nil {
		return "", err
	}
	return repositoryUri, nil
}
