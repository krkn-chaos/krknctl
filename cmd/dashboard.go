package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	orcmodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	"github.com/spf13/cobra"
)

// Default in-container paths for the stock krkn-dashboard image (override with flags).
const defaultDashboardChaosAssetsDir = "/usr/src/chaos-dashboard/src/assets"
const defaultDashboardDatabaseMount = "/usr/src/chaos-dashboard/database"

// dashboardKubeconfigMountPath returns the in-container kubeconfig file path under chaos assets.
func dashboardKubeconfigMountPath(chaosAssetsDir string) string {
	return path.Join(chaosAssetsDir, "kubeconfig")
}

func dashboardPublishPorts() []string {
	return []string{
		"127.0.0.1:3000:3000",
		"127.0.0.1:8000:8000",
	}
}

func hostSocketPath(socketURI string) string {
	p := strings.TrimPrefix(socketURI, "unix://")
	if p == socketURI {
		p = strings.TrimPrefix(socketURI, "unix:")
	}
	if p != "" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func NewDashboardCommand(scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, cfg config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "dashboard",
		Short: "runs the krkn-dashboard web UI in a container",
		Long: `Runs the krkn-dashboard container with the container runtime socket mounted and a kubeconfig file bind-mounted into the dashboard.

--kubeconfig is required: pass the host path to the kubeconfig file to use inside the dashboard.

The kubeconfig bind mount targets {chaos-assets-dir}/kubeconfig only (not the whole src/assets tree). CHAOS_ASSETS is set to --chaos-assets-dir. KUBECONFIG_PATH is the absolute host path to the prepared kubeconfig (same as the bind source for the dashboard mount, suitable as a host bind source for nested podman: often a flattened temp file when PrepareKubeconfig inlined certs/keys).

Optional --chaos-assets-dir and --database-dir default to the stock image paths. Operators configure host paths; in-container paths match the image unless overridden.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
			podmanPlatform, _ := cmd.Flags().GetString("podman-platform")
			chaosAssetsDir, _ := cmd.Flags().GetString("chaos-assets-dir")
			databaseDir, _ := cmd.Flags().GetString("database-dir")
			kubeconfigMount := dashboardKubeconfigMountPath(chaosAssetsDir)

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dataDir := filepath.Join(home, ".krknctl", "krkn-dashboard-data")

			(*scenarioOrchestrator).PrintContainerRuntime()

			kube := strings.TrimSpace(kubeconfigPath)
			if kube == "" {
				return fmt.Errorf("--kubeconfig is required")
			}
			preparedKubeconfig, err := utils.PrepareKubeconfig(&kube, cfg)
			if err != nil {
				return err
			}
			if preparedKubeconfig == nil {
				return fmt.Errorf("kubeconfig not found or unreadable at %q", kube)
			}
			defer func() { _ = os.Remove(*preparedKubeconfig) }()

			kubeconfigBindAbs, err := filepath.Abs(*preparedKubeconfig)
			if err != nil {
				return err
			}

			dataAbs, err := filepath.Abs(dataDir)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dataAbs, 0750); err != nil {
				return err
			}

			socketURI, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return err
			}
			hostSock := hostSocketPath(*socketURI)
			if hostSock == "" {
				return fmt.Errorf("could not resolve container runtime socket path from %q", *socketURI)
			}

			containerRuntime := (*scenarioOrchestrator).GetContainerRuntime()

			volumes := map[string]string{
				kubeconfigBindAbs: kubeconfigMount,
				dataAbs:           databaseDir,
			}
			displayEnv := map[string]ParsedField{
				"CHAOS_ASSETS":    {value: fmt.Sprintf("%s (in-container); kubeconfig bind %s → %s", chaosAssetsDir, kubeconfigBindAbs, kubeconfigMount)},
				"KUBECONFIG_PATH": {value: fmt.Sprintf("%s (host path, bind source; same as for nested -v) → %s in container", kubeconfigBindAbs, kubeconfigMount)},
			}
			environment := map[string]string{
				"CHAOS_ASSETS":    chaosAssetsDir,
				"KUBECONFIG_PATH": kubeconfigBindAbs,
			}

			resolvedPodmanPlatform := podmanPlatform
			if containerRuntime == orcmodels.Podman && runtime.GOOS == "darwin" && resolvedPodmanPlatform == "" {
				resolvedPodmanPlatform = "linux/amd64"
			}
			if resolvedPodmanPlatform != "" {
				environment["PODMAN_PLATFORM"] = resolvedPodmanPlatform
				displayEnv["PODMAN_PLATFORM"] = ParsedField{value: resolvedPodmanPlatform}
			}

			switch containerRuntime {
			case orcmodels.Podman:
				volumes[hostSock] = "/run/podman/podman.sock"
			case orcmodels.Docker:
				volumes[hostSock] = "/var/run/docker.sock"
				environment["CONTAINER_HOST"] = "unix:///var/run/docker.sock"
				displayEnv["CONTAINER_HOST"] = ParsedField{value: "unix:///var/run/docker.sock"}
			default:
				return fmt.Errorf("unsupported container runtime for dashboard: %s", containerRuntime.String())
			}

			var podmanCreate *scenarioorchestrator.PodmanCreateOptions
			if containerRuntime == orcmodels.Podman {
				podmanCreate = &scenarioorchestrator.PodmanCreateOptions{
					SecurityOpts: []string{"label=disable"},
				}
				if resolvedPodmanPlatform != "" {
					podmanCreate.ImagePlatform = resolvedPodmanPlatform
				} else if podmanPlatform != "" {
					podmanCreate.ImagePlatform = podmanPlatform
				}
				if gid, err := podmanAPISocketGID(hostSock); err == nil && gid != "" {
					podmanCreate.GroupAdd = []string{gid}
				}
			}

			NewEnvironmentTable(displayEnv, cfg).Print()

			conn, err := (*scenarioOrchestrator).Connect(*socketURI)
			if err != nil {
				return err
			}

			containerName := utils.GenerateContainerName(cfg, "krkn-dashboard", nil)

			spinner := NewSpinnerWithSuffix(" pulling krkn-dashboard image...")
			spinner.Start()
			commChan := make(chan *string)
			go func() {
				for msg := range commChan {
					spinner.Suffix = " " + *msg
				}
				spinner.Stop()
			}()

			dashboardImage, err := cfg.GetDashboardImageURI()
			if err != nil {
				return err
			}
			_, _ = color.New(color.FgYellow).Println("Starting krkn-dashboard - open http://localhost:3000 when the app is ready (Ctrl+C to stop).")
			_, err = (*scenarioOrchestrator).RunAttached(dashboardImage, containerName, environment, false, volumes, os.Stdout, os.Stderr, &commChan, conn, nil, dashboardPublishPorts(), podmanCreate)
			if err != nil {
				return err
			}

			_, err = color.New(color.FgGreen).Println("krkn-dashboard stopped. While it was running, the UI was at http://localhost:3000")
			return err
		},
	}

	command.Flags().String("kubeconfig", "", "host path to kubeconfig file (required)")
	command.Flags().String("chaos-assets-dir", defaultDashboardChaosAssetsDir, "in-container CHAOS_ASSETS directory")
	command.Flags().String("database-dir", defaultDashboardDatabaseMount, "in-container mount path for dashboard SQLite data")
	command.Flags().String("podman-platform", "", "platform for the dashboard image (PODMAN_PLATFORM env); default linux/amd64 on macOS Podman")
	if err := command.MarkFlagRequired("kubeconfig"); err != nil {
		panic(fmt.Sprintf("dashboard: MarkFlagRequired(kubeconfig): %v", err))
	}

	return command
}
