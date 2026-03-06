package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const visualizeRepoURL = "https://github.com/krkn-chaos/visualize.git"

func NewVisualizeCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "visualize",
		Short: "deploys krkn-visualize (Grafana dashboard) to the current Kubernetes cluster",
		Long:  `deploys krkn-visualize (Grafana dashboard) with Elasticsearch and optional Prometheus datasources to the current Kubernetes cluster`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			esURL, _ := cmd.Flags().GetString("es-url")
			esUsername, _ := cmd.Flags().GetString("es-username")
			esPasswordRaw, _ := cmd.Flags().GetString("es-password")
			esPassword := ParsedField{value: esPasswordRaw, secret: true}
			prometheusURL, _ := cmd.Flags().GetString("prometheus-url")
			prometheusBearer, _ := cmd.Flags().GetString("prometheus-bearer-token")
			namespace, _ := cmd.Flags().GetString("namespace")
			grafanaPassword, _ := cmd.Flags().GetString("grafana-password")
			kubectlCmd, _ := cmd.Flags().GetString("kubectl")
			deleteFlag, _ := cmd.Flags().GetBool("delete")

			if _, err := exec.LookPath("git"); err != nil {
				return fmt.Errorf("git is required but not found in PATH")
			}
			if _, err := exec.LookPath("bash"); err != nil {
				return fmt.Errorf("bash is required but not found in PATH")
			}

			tmpDir, err := os.MkdirTemp("", "krkn-visualize-*")
			if err != nil {
				return fmt.Errorf("failed to create temp directory: %w", err)
			}
			defer os.RemoveAll(tmpDir) // #nosec G703

			spinner := NewSpinnerWithSuffix(" cloning krkn-visualize repository...")
			spinner.Start()
			cloneCmd := exec.Command("git", "clone", "--depth=1", visualizeRepoURL, tmpDir) // #nosec G204
			cloneCmd.Stderr = os.Stderr
			if err := cloneCmd.Run(); err != nil {
				spinner.Stop()
				return fmt.Errorf("failed to clone visualize repository: %w", err)
			}
			spinner.Stop()

			deployScript := filepath.Join(tmpDir, "krkn-visualize", "deploy.sh")
			if _, err := os.Stat(deployScript); os.IsNotExist(err) {
				return fmt.Errorf("deploy.sh not found in cloned visualize repository")
			}

			deployArgs := []string{deployScript}
			if namespace != "" {
				deployArgs = append(deployArgs, "-n", namespace)
			}
			if grafanaPassword != "" {
				deployArgs = append(deployArgs, "-p", grafanaPassword)
			}
			if kubectlCmd != "" {
				deployArgs = append(deployArgs, "-c", kubectlCmd)
			}
			if prometheusURL != "" {
				deployArgs = append(deployArgs, "-t", prometheusURL)
			}
			if prometheusBearer != "" {
				deployArgs = append(deployArgs, "-b", prometheusBearer)
			}
			if deleteFlag {
				deployArgs = append(deployArgs, "-d")
			}

			env := os.Environ()
			if esURL != "" {
				env = append(env, "ES_URL="+esURL)
			}
			if esUsername != "" {
				env = append(env, "ES_USERNAME="+esUsername)
			}
			if esPassword.value != "" {
				env = append(env, "ES_PASSWORD="+esPassword.value)
			}
			if prometheusURL != "" {
				env = append(env, "PROMETHEUS_URL="+prometheusURL)
			}
			if prometheusBearer != "" {
				env = append(env, "PROMETHEUS_BEARER="+prometheusBearer)
			}

			deployCmd := exec.Command("bash", deployArgs...) // #nosec G204
			deployCmd.Stdout = os.Stdout
			deployCmd.Stderr = os.Stderr
			deployCmd.Env = env
			deployCmd.Dir = filepath.Join(tmpDir, "krkn-visualize")

			if err := deployCmd.Run(); err != nil {
				return fmt.Errorf("krkn-visualize deployment failed: %w", err)
			}

			_, err = color.New(color.FgGreen).Println("krkn-visualize deployed successfully!")
			return err
		},
	}

	command.Flags().String("es-url", "", "Elasticsearch URL")
	command.Flags().String("es-username", "", "Elasticsearch username")
	command.Flags().String("es-password", "", "Elasticsearch password")
	command.Flags().String("prometheus-url", "", "Prometheus URL for datasource (optional)")
	command.Flags().String("prometheus-bearer-token", "", "Prometheus bearer token for authentication (optional)")
	command.Flags().String("namespace", "krkn-visualize", "Kubernetes namespace to deploy into")
	command.Flags().String("grafana-password", "admin", "Grafana admin password")
	command.Flags().String("kubectl", "kubectl", "kubectl command to use (e.g. oc for OpenShift)")
	command.Flags().Bool("delete", false, "delete an existing krkn-visualize deployment")

	return command
}
