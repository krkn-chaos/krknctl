package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDashboardCommand(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)
	assert.NotNil(t, cmd)
	assert.Equal(t, "dashboard", cmd.Use)
	assert.Contains(t, cmd.Long, "--kubeconfig is required")
	assert.Contains(t, cmd.Long, "KUBECONFIG_PATH")
	assert.Contains(t, cmd.Long, "--chaos-assets-dir")
}

func TestDashboardCommandFlags(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)

	assert.NotNil(t, cmd.Flags().Lookup("kubeconfig"))
	assert.NotNil(t, cmd.Flags().Lookup("chaos-assets-dir"))
	assert.NotNil(t, cmd.Flags().Lookup("database-dir"))
	assert.NotNil(t, cmd.Flags().Lookup("podman-platform"))
}

func TestDashboardCommandFlagDefaults(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	assert.Nil(t, err)
	assert.Equal(t, "", kubeconfig)

	platform, err := cmd.Flags().GetString("podman-platform")
	assert.Nil(t, err)
	assert.Equal(t, "", platform)

	chaosAssets, err := cmd.Flags().GetString("chaos-assets-dir")
	assert.Nil(t, err)
	assert.Equal(t, defaultDashboardChaosAssetsDir, chaosAssets)

	dbDir, err := cmd.Flags().GetString("database-dir")
	assert.Nil(t, err)
	assert.Equal(t, defaultDashboardDatabaseMount, dbDir)
}

func TestDashboardKubeconfigMountPath(t *testing.T) {
	assert.Equal(t, "/usr/src/chaos-dashboard/src/assets/kubeconfig", dashboardKubeconfigMountPath("/usr/src/chaos-dashboard/src/assets"))
	assert.Equal(t, "/custom/assets/kubeconfig", dashboardKubeconfigMountPath("/custom/assets"))
}

func TestDashboardPublishPortsLoopback(t *testing.T) {
	ports := dashboardPublishPorts()
	assert.Equal(t, []string{"127.0.0.1:3000:3000", "127.0.0.1:8000:8000"}, ports)
}

func TestHostSocketPath(t *testing.T) {
	assert.Equal(t, "/var/run/docker.sock", hostSocketPath("unix:///var/run/docker.sock"))
	assert.Equal(t, "/run/user/501/podman/podman.sock", hostSocketPath("unix://run/user/501/podman/podman.sock"))
	assert.Equal(t, "/run/podman/podman.sock", hostSocketPath("unix://run/podman/podman.sock"))
}
