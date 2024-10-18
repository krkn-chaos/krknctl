package container_manager

type Environment int64

const (
	Podman Environment = iota
	Docker
)

type ContainerManager interface {
	Run(image string,
		scenarioName string,
		containerRuntimeUri string,
		env map[string]string,
		cache bool,
		volumeMounts map[string]string,
		localKubeconfigPath string,
		kubeconfigMountPath string,

	) (*string, error)
}
