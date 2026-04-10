package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDashboardCommand(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)
	assert.NotNil(t, cmd)
	assert.Equal(t, "dashboard", cmd.Use)
}

func TestDashboardCommandFlags(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)

	assert.NotNil(t, cmd.Flags().Lookup("kubeconfig"))
	assert.NotNil(t, cmd.Flags().Lookup("data-dir"))
	assert.NotNil(t, cmd.Flags().Lookup("podman-platform"))
	assert.NotNil(t, cmd.Flags().Lookup("bind-all-interfaces"))
}

func TestDashboardCommandFlagDefaults(t *testing.T) {
	orchestrator := getOrchestrator(t)
	config := getConfig(t)
	cmd := NewDashboardCommand(&orchestrator, config)
	if cmd.PreRunE != nil {
		err := cmd.PreRunE(cmd, []string{})
		assert.Nil(t, err)
	}

	dataDir, err := cmd.Flags().GetString("data-dir")
	assert.Nil(t, err)
	assert.True(t, strings.HasSuffix(dataDir, filepath.Join(".krknctl", "krkn-dashboard-data")))

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	assert.Nil(t, err)
	assert.Equal(t, "", kubeconfig)

	platform, err := cmd.Flags().GetString("podman-platform")
	assert.Nil(t, err)
	assert.Equal(t, "", platform)

	bindAll, err := cmd.Flags().GetBool("bind-all-interfaces")
	assert.Nil(t, err)
	assert.False(t, bindAll)
}

func TestDashboardPublishPortsLoopback(t *testing.T) {
	ports := dashboardPublishPorts(false)
	assert.Equal(t, []string{"127.0.0.1:3000:3000", "127.0.0.1:8000:8000"}, ports)
}

func TestDashboardPublishPortsAllInterfaces(t *testing.T) {
	ports := dashboardPublishPorts(true)
	assert.Equal(t, []string{"0.0.0.0:3000:3000", "0.0.0.0:8000:8000"}, ports)
}

func TestHostSocketPath(t *testing.T) {
	assert.Equal(t, "/var/run/docker.sock", hostSocketPath("unix:///var/run/docker.sock"))
	assert.Equal(t, "/run/user/501/podman/podman.sock", hostSocketPath("unix://run/user/501/podman/podman.sock"))
	assert.Equal(t, "/run/podman/podman.sock", hostSocketPath("unix://run/podman/podman.sock"))
}
