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

func podmanSocketProbeScript() string {
	parts := []string{
		`for p in /run/podman/podman.sock /run/user/$(id -u)/podman/podman.sock; do`,
		`[ -S "$p" ] && printf %s "$p" && exit 0;`,
		`done;`,
		`f=$(find /run /var/run -type s -name podman.sock 2>/dev/null | head -n1);`,
		`[ -n "$f" ] && [ -S "$f" ] && printf %s "$f" && exit 0;`,
		`exit 1`,
	}
	return strings.Join(parts, " ")
}

func podmanRunningMachineSocketPath(machine string) (string, error) {
	cmd := exec.Command("podman", "machine", "ssh", machine, podmanSocketProbeScript()) 
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("podman machine ssh socket probe: %w: %s", err, msg)
		}
		return "", err
	}
	socketPath := strings.TrimSpace(string(out))
	if socketPath == "" {
		return "", fmt.Errorf("podman machine ssh socket probe returned empty path")
	}
	return socketPath, nil
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
func podmanAPISocketGID(hostSocketPath string) (string, error) {
	if runtime.GOOS == "darwin" {
		machine, err := podmanRunningMachineName()
		if err != nil {
			return "", err
		}
		socketPath, err := podmanRunningMachineSocketPath(machine)
		if err != nil {
			return "", err
		}
		cmd := exec.Command("podman", "machine", "ssh", machine, "stat", "-c", "%g", socketPath) // #nosec G204 -- fixed podman/stat argv and socket path probe; machine name from podman machine list JSON only
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return "", fmt.Errorf("podman machine ssh stat %s: %w: %s", socketPath, err, msg)
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
