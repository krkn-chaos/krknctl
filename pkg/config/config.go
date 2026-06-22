// Package config provides all the string constraints and options for the krknctl tool
// Assisted by Claude Sonnet 4
package config

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/url"

	"github.com/krkn-chaos/krknctl/pkg/gpudetect"
)

type Config struct {
	Version                          string `json:"version"`
	QuayHost                         string `json:"quay_host"`
	QuayOrg                          string `json:"quay_org"`
	QuayScenarioRegistry             string `json:"quay_scenario_registry"`
	QuayBaseImageRegistry            string `json:"quay_base_image_registry"`
	QuayBaseImageTag                 string `json:"quay_base_image_tag"`
	QuayRepositoryAPI                string `json:"quay_repository_api"`
	CustomDomainHost                 string `json:"custom_domain_host"`
	PrivateRegistryBaseImageTag      string `json:"private_registry_base_image_tag"`
	ContainerPrefix                  string `json:"container_prefix"`
	KubeconfigPrefix                 string `json:"kubeconfig_prefix"`
	PodmanDarwinSocketTemplate       string `json:"podman_darwin_socket_template"`
	PodmanLinuxSocketTemplate        string `json:"podman_linux_socket_template"`
	PodmanSocketRoot                 string `json:"podman_socket_root_linux"`
	PodmanRunningState               string `json:"podman_running_state"`
	DockerSocketRoot                 string `json:"docker_socket_root"`
	DockerRunningState               string `json:"docker_running_state"`
	DefaultContainerPlatform         string `json:"default_container_platform"`
	MetricsProfilePath               string `json:"metrics_profile_path"`
	AlertsProfilePath                string `json:"alerts_profile_path"`
	KubeconfigPath                   string `json:"kubeconfig_path"`
	RandomGraphPath                  string `json:"random_graph_path"`
	LabelTitle                       string `json:"label_title"`
	LabelDescription                 string `json:"label_description"`
	LabelInputFields                 string `json:"label_input_fields"`
	LabelTitleGlobal                 string `json:"label_title_global"`
	LabelDescriptionGlobal           string `json:"label_description_global"`
	LabelInputFieldsGlobal           string `json:"label_input_fields_global"`
	LabelTitleRegex                  string `json:"label_title_regex"`
	LabelDescriptionRegex            string `json:"label_description_regex"`
	LabelInputFieldsRegex            string `json:"label_input_fields_regex"`
	LabelTitleRegexGlobal            string `json:"label_title_regex_global"`
	LabelDescriptionRegexGlobal      string `json:"label_description_regex_global"`
	LabelInputFieldsRegexGlobal      string `json:"label_input_fields_regex_global"`
	LabelRootNode                    string `json:"label_root_node"`
	EnvPrivateRegistry               string `json:"env_private_registry"`
	EnvPrivateRegistryUsername       string `json:"env_private_registry_username"`
	EnvPrivateRegistryPassword       string `json:"env_private_registry_password"`
	EnvPrivateRegistrySkipTLS        string `json:"env_private_registry_skip_tls"`
	EnvPrivateRegistryToken          string `json:"env_private_registry_token"`
	EnvPrivateRegistryScenarios      string `json:"env_private_registry_scenarios"`
	EnvPrivateRegistryInsecure       string `json:"env_private_registry_insecure"`
	GithubLatestRelease              string `json:"github_latest_release"`
	GithubLatestReleaseAPI           string `json:"github_latest_release_api"`
	GithubReleaseAPI                 string `json:"github_release_api"`
	GithubReleaseAPIDeprecated       string `json:"github_release_api_deprecated"`
	TableFieldMaxLength              int    `json:"table_field_max_length"`
	TableMaxStepScenarioLength       int    `json:"table_max_step_scenario_length"`
	AssistRegistry                   string `json:"assist_registry"`
	AssistModelTagApple              string `json:"assist_model_tag_apple"`
	AssistModelTagNvidiaConsumer     string `json:"assist_model_tag_nvidia_consumer"`
	AssistModelTagNvidiaDatacenter   string `json:"assist_model_tag_nvidia_datacenter"`
	AssistModelTagCPU                string `json:"assist_model_tag_cpu"`
	AssistContainerPrefix            string `json:"assist_container_prefix"`
	AssistServicePort                string `json:"assist_service_port"`
	AssistHealthEndpoint             string `json:"assist_health_endpoint"`
	AssistQueryEndpoint              string `json:"assist_query_endpoint"`
	AssistHost                       string `json:"assist_host"`
	AssistHealthMaxRetries           int    `json:"assist_health_max_retries"`
	AssistHealthRetryIntervalSeconds int    `json:"assist_health_retry_interval_seconds"`
	AssistQueryMaxResults            int    `json:"assist_query_max_results"`
	VisualizeImageName               string `json:"visualize_image_name"`
	VisualizeImageTag                string `json:"visualize_image_tag"`
	DashboardImageName               string `json:"dashboard_image_name"`
	DashboardImageTag                string `json:"dashboard_image_tag"`
	LabelIsAScenarioRegex            string `json:"label_is_a_scenario_regex"`
	LabelHasRollbackRegex            string `json:"label_has_rollback_regex"`
	LabelIsAScenario                 string `json:"label_is_a_scenario"`
	LabelHasRollback                 string `json:"label_has_rollback"`
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

func (c *Config) GetQuayImageURI() (string, error) {
	imageURI, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.QuayScenarioRegistry)
	if err != nil {
		return "", err
	}
	return imageURI, nil
}

func (c *Config) GetCustomDomainImageURI() (string, error) {
	imageURI, err := url.JoinPath(c.CustomDomainHost, c.QuayOrg, c.QuayScenarioRegistry)
	if err != nil {
		return "", err
	}
	return imageURI, nil
}

func (c *Config) GetQuayScenarioRepositoryAPIURI() (string, error) {
	baseHost := "https://" + c.QuayHost
	repositoryURI, err := url.JoinPath(baseHost, c.QuayRepositoryAPI, c.QuayOrg, c.QuayScenarioRegistry)
	if err != nil {
		return "", err
	}
	return repositoryURI, nil
}

func (c *Config) GetVisualizeImageURI() (string, error) {
	imageURI, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.VisualizeImageName)
	if err != nil {
		return "", err
	}
	return imageURI + ":" + c.VisualizeImageTag, nil
}

func (c *Config) GetDashboardImageURI() (string, error) {
	imageURI, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.DashboardImageName)
	if err != nil {
		return "", err
	}
	return imageURI + ":" + c.DashboardImageTag, nil
}

func (c *Config) GetQuayBaseImageRepositoryAPIURI() (string, error) {
	baseHost := "https://" + c.QuayHost
	repositoryURI, err := url.JoinPath(baseHost, c.QuayRepositoryAPI, c.QuayOrg, c.QuayBaseImageRegistry)
	if err != nil {
		return "", err
	}
	return repositoryURI, nil
}

// GetAssistImageURI returns the assist container image URI based on detected GPU type
func (c *Config) GetAssistImageURI() (string, error) {
	imageURI, err := url.JoinPath(c.QuayHost, c.QuayOrg, c.AssistRegistry)
	if err != nil {
		return "", err
	}

	gpuType, err := gpudetect.DetectGPU()
	if err != nil {
		log.Printf("Warning: GPU detection failed: %v, using CPU-only mode", err)
		gpuType = gpudetect.GPUTypeCPU
	}

	var tag string
	switch gpuType {
	case gpudetect.GPUTypeAppleSilicon:
		tag = c.AssistModelTagApple
	case gpudetect.GPUTypeNvidiaConsumer:
		tag = c.AssistModelTagNvidiaConsumer
	case gpudetect.GPUTypeNvidiaDatacenter:
		tag = c.AssistModelTagNvidiaDatacenter
	case gpudetect.GPUTypeCPU:
		tag = c.AssistModelTagCPU
	default:
		tag = c.AssistModelTagCPU
	}

	return imageURI + ":" + tag, nil
}

// GetAssistImageURIWithRegistry returns the assist container image URI from a custom registry
// If registryURL is empty, falls back to public registry
func (c *Config) GetAssistImageURIWithRegistry(registryURL string, assistRepo string) (string, error) {
	// If no custom registry, use default public registry
	if registryURL == "" {
		return c.GetAssistImageURI()
	}

	// Use assist repository from parameter, or fall back to config default
	repo := assistRepo
	if repo == "" {
		repo = c.AssistRegistry
	}

	// Build image URI with custom registry
	imageURI, err := url.JoinPath(registryURL, repo)
	if err != nil {
		return "", err
	}

	// Detect GPU type
	gpuType, err := gpudetect.DetectGPU()
	if err != nil {
		log.Printf("Warning: GPU detection failed: %v, using CPU-only mode", err)
		gpuType = gpudetect.GPUTypeCPU
	}

	var tag string
	switch gpuType {
	case gpudetect.GPUTypeAppleSilicon:
		tag = c.AssistModelTagApple
	case gpudetect.GPUTypeNvidiaConsumer:
		tag = c.AssistModelTagNvidiaConsumer
	case gpudetect.GPUTypeNvidiaDatacenter:
		tag = c.AssistModelTagNvidiaDatacenter
	case gpudetect.GPUTypeCPU:
		tag = c.AssistModelTagCPU
	default:
		tag = c.AssistModelTagCPU
	}

	return imageURI + ":" + tag, nil
}
