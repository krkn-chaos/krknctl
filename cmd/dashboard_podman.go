package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type podmanMachineListEntry struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
}

func podmanRunningMachineName() (string, error) {
	out, err := exec.Command("podman", "machine", "list", "--format", "json").Output()
	if err != nil {
		return "", err
	}
	var rows []podmanMachineListEntry
	if err := json.Unmarshal(out, &rows); err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Running {
			return strings.TrimSuffix(strings.TrimSpace(r.Name), "*"), nil
		}
	}
	return "", fmt.Errorf("no running podman machine")
}

// podmanAPISocketGID returns the numeric GID of the Podman API socket for supplemental group membership
// inside the dashboard container (avoids EACCES on /run/podman/podman.sock per podman-run.sh).
func podmanAPISocketGID(hostSocketPath string) (string, error) {
	if runtime.GOOS == "darwin" {
		machine, err := podmanRunningMachineName()
		if err != nil {
			return "", err
		}
		cmd := exec.Command("podman", "machine", "ssh", machine, "stat", "-c", "%g", "/run/podman/podman.sock") // #nosec G204 -- fixed podman/stat argv; machine name from podman machine list JSON only
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return "", fmt.Errorf("podman machine ssh stat: %w: %s", err, msg)
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
	cmd := exec.Command("stat", "-c", "%g", hostSocketPath) // #nosec G204 -- stat with path from resolved container runtime socket URI, not shell input
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
