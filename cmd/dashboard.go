package cmd

import (
	"fmt"
	"io"
	"os"
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

// Mount kubeconfig as a single file so we do not replace the image's entire src/assets tree
// (Vite imports such as @/assets/scenario-icons/*.jpg would break if src/assets were a host dir).
const dashboardKubeconfigMount = "/usr/src/chaos-dashboard/src/assets/kubeconfig"
const dashboardDatabaseMount = "/usr/src/chaos-dashboard/database"

// In-container assets dir (uploads, etc.). Podman -v for krkn-hub must use the host path in
// KRKN_DASHBOARD_KUBECONFIG_BIND_SRC — see server start-kraken.
const dashboardChaosAssetsDir = "/usr/src/chaos-dashboard/src/assets"

func dashboardPublishPorts(bindAllInterfaces bool) []string {
	host := "127.0.0.1"
	if bindAllInterfaces {
		host = "0.0.0.0"
	}
	return []string{
		fmt.Sprintf("%s:3000:3000", host),
		fmt.Sprintf("%s:8000:8000", host),
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

When --kubeconfig is omitted, the kubeconfig path is resolved the same way as kubectl (KUBECONFIG if set, otherwise the default file such as ~/.kube/config). Pass --kubeconfig explicitly to use a specific file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
			dataDir, _ := cmd.Flags().GetString("data-dir")
			podmanPlatform, _ := cmd.Flags().GetString("podman-platform")
			bindAll, _ := cmd.Flags().GetBool("bind-all-interfaces")

			(*scenarioOrchestrator).PrintContainerRuntime()

			var foundKubeconfig *string
			if kubeconfigPath != "" {
				foundKubeconfig = &kubeconfigPath
			}
			preparedKubeconfig, err := utils.PrepareKubeconfig(foundKubeconfig, cfg)
			if err != nil {
				return err
			}
			if preparedKubeconfig == nil {
				return fmt.Errorf("kubeconfig not found at the default path or the path given by --kubeconfig; ensure the file exists or set --kubeconfig to a valid kubeconfig")
			}
			defer func() { _ = os.Remove(*preparedKubeconfig) }()

			assetsDir, err := os.MkdirTemp("", "krknctl-dashboard-assets-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(assetsDir) }()

			kubeconfigDest := filepath.Join(assetsDir, "kubeconfig")
			if err := copyFile(*preparedKubeconfig, kubeconfigDest); err != nil {
				return err
			}
			kubeconfigDestAbs, err := filepath.Abs(kubeconfigDest)
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
				kubeconfigDest: dashboardKubeconfigMount,
				dataAbs:        dashboardDatabaseMount,
			}

			displayEnv := map[string]ParsedField{
				"CHAOS_ASSETS":                       {value: fmt.Sprintf("%s (file → %s)", kubeconfigDestAbs, dashboardKubeconfigMount)},
				"KRKN_DASHBOARD_KUBECONFIG_BIND_SRC": {value: kubeconfigDestAbs},
			}
			environment := map[string]string{
				"CHAOS_ASSETS":                       dashboardChaosAssetsDir,
				"KRKN_DASHBOARD_KUBECONFIG_BIND_SRC": kubeconfigDestAbs,
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
			if containerRuntime == orcmodels.Podman && runtime.GOOS == "darwin" {
				podmanCreate = &scenarioorchestrator.PodmanCreateOptions{
					ImagePlatform: resolvedPodmanPlatform,
					SecurityOpts:  []string{"label=disable"},
				}
				if gid, err := podmanAPISocketGID(hostSock); err == nil && gid != "" {
					podmanCreate.GroupAdd = []string{gid}
				}
			} else if containerRuntime == orcmodels.Podman && podmanPlatform != "" {
				podmanCreate = &scenarioorchestrator.PodmanCreateOptions{ImagePlatform: podmanPlatform}
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
			if bindAll {
				_, _ = color.New(color.FgYellow).Println("Dashboard ports bind to all interfaces (0.0.0.0); reachable from the network.")
			}
			_, _ = color.New(color.FgYellow).Println("Starting krkn-dashboard - open http://localhost:3000 when the app is ready (Ctrl+C to stop).")
			_, err = (*scenarioOrchestrator).RunAttached(dashboardImage, containerName, environment, false, volumes, os.Stdout, os.Stderr, &commChan, conn, nil, dashboardPublishPorts(bindAll), podmanCreate)
			if err != nil {
				return err
			}

			_, err = color.New(color.FgGreen).Println("krkn-dashboard stopped. While it was running, the UI was at http://localhost:3000")
			return err
		},
	}

	command.Flags().String("kubeconfig", "", "path to kubeconfig file; if omitted, uses kubectl-style default resolution (e.g. KUBECONFIG or ~/.kube/config)")
	command.Flags().String("data-dir", "", "host directory for dashboard SQLite data (defaults to ~/.krknctl/krkn-dashboard-data)")
	command.Flags().String("podman-platform", "", "platform for the dashboard image (PODMAN_PLATFORM env); default linux/amd64 on macOS Podman")
	command.Flags().Bool("bind-all-interfaces", false, "publish dashboard ports on 0.0.0.0 instead of localhost only (reachable from the network)")

	command.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("data-dir") {
			return nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return cmd.Flags().Set("data-dir", filepath.Join(home, ".krknctl", "krkn-dashboard-data"))
	}

	return command
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- paths are PrepareKubeconfig temp file and dashboard MkdirTemp assets dir, not arbitrary traversal
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // #nosec G304 -- dst is always under krknctl-created temp dir for dashboard bind mounts
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}
