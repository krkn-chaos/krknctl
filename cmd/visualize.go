package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/spf13/cobra"
)

func NewVisualizeCommand(scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, config config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "visualize",
		Short: "deploys krkn-visualize (Grafana dashboard) to the current Kubernetes cluster",
		Long:  `deploys krkn-visualize (Grafana dashboard) with Elasticsearch and optional Prometheus datasources to the current Kubernetes cluster`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			esURL, _ := cmd.Flags().GetString("es-url")
			esUsername, _ := cmd.Flags().GetString("es-username")
			esPasswordRaw, _ := cmd.Flags().GetString("es-password")
			prometheusURL, _ := cmd.Flags().GetString("prometheus-url")
			prometheusBearerRaw, _ := cmd.Flags().GetString("prometheus-bearer-token")
			namespace, _ := cmd.Flags().GetString("namespace")
			grafanaPassword, _ := cmd.Flags().GetString("grafana-password")
			kubectlCmd, _ := cmd.Flags().GetString("kubectl")
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
			deleteFlag, _ := cmd.Flags().GetBool("delete")

			(*scenarioOrchestrator).PrintContainerRuntime()

			var foundKubeconfig *string
			if kubeconfigPath != "" {
				foundKubeconfig = &kubeconfigPath
			}
			preparedKubeconfig, err := utils.PrepareKubeconfig(foundKubeconfig, config)
			if err != nil {
				return err
			}
			if preparedKubeconfig == nil {
				return fmt.Errorf("kubeconfig not found, please specify a valid kubeconfig path with --kubeconfig")
			}

			volumes := map[string]string{
				*preparedKubeconfig: config.KubeconfigPath,
			}

			displayEnv := map[string]ParsedField{
				"KUBECONFIG": {value: config.KubeconfigPath},
				"K8S_CMD":    {value: kubectlCmd},
				"NAMESPACE":  {value: namespace},
			}
			environment := map[string]string{
				"KUBECONFIG": config.KubeconfigPath,
				"K8S_CMD":    kubectlCmd,
				"NAMESPACE":  namespace,
			}
			if grafanaPassword != "" {
				environment["GRAFANA_PASSWORD"] = grafanaPassword
				displayEnv["GRAFANA_PASSWORD"] = ParsedField{value: grafanaPassword}
			}
			if deleteFlag {
				environment["DELETE"] = "true"
				displayEnv["DELETE"] = ParsedField{value: "true"}
			}
			if esURL != "" {
				environment["ES_URL"] = esURL
				displayEnv["ES_URL"] = ParsedField{value: esURL}
			}
			if esUsername != "" {
				environment["ES_USERNAME"] = esUsername
				displayEnv["ES_USERNAME"] = ParsedField{value: esUsername}
			}
			if esPasswordRaw != "" {
				environment["ES_PASSWORD"] = esPasswordRaw
				displayEnv["ES_PASSWORD"] = ParsedField{value: esPasswordRaw, secret: true}
			}
			if prometheusURL != "" {
				environment["PROMETHEUS_URL"] = prometheusURL
				displayEnv["PROMETHEUS_URL"] = ParsedField{value: prometheusURL}
			}
			if prometheusBearerRaw != "" {
				environment["PROMETHEUS_BEARER"] = prometheusBearerRaw
				displayEnv["PROMETHEUS_BEARER"] = ParsedField{value: prometheusBearerRaw, secret: true}
			}

			NewEnvironmentTable(displayEnv, config).Print()

			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			conn, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return err
			}

			containerName := utils.GenerateContainerName(config, "krkn-visualize", nil)

			spinner := NewSpinnerWithSuffix(" pulling krkn-visualize image...")
			spinner.Start()
			commChan := make(chan *string)
			go func() {
				for msg := range commChan {
					spinner.Suffix = " " + *msg
				}
				spinner.Stop()
			}()

			visualizeImage, err := config.GetVisualizeImageURI()
			if err != nil {
				return err
			}
			_, err = (*scenarioOrchestrator).RunAttached(visualizeImage, containerName, environment, false, volumes, os.Stdout, os.Stderr, &commChan, conn, nil)
			if err != nil {
				return err
			}

			_, err = color.New(color.FgGreen).Println("krkn-visualize completed successfully!")
			return err
		},
	}

	command.Flags().String("es-url", "", "Elasticsearch URL")
	command.Flags().String("es-username", "", "Elasticsearch username")
	command.Flags().String("es-password", "", "Elasticsearch password")
	command.Flags().String("prometheus-url", "", "Prometheus URL for datasource (optional)")
	command.Flags().String("prometheus-bearer-token", "", "Prometheus bearer token for authentication (optional)")
	command.Flags().String("namespace", "krkn-visualize", "Kubernetes namespace to deploy into")
	command.Flags().String("grafana-password", "", "Grafana admin password")
	_ = command.MarkFlagRequired("grafana-password")
	command.Flags().String("kubectl", "kubectl", "kubectl command to use (e.g. oc for OpenShift)")
	command.Flags().String("kubeconfig", "", "kubeconfig path (defaults to ~/.kube/config)")
	command.Flags().Bool("delete", false, "delete an existing krkn-visualize deployment")

	return command
}
