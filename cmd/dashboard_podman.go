package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// podmanMachineNameRe matches names returned by `podman machine list` (no shell metacharacters).
var podmanMachineNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// podmanSocketPathRe matches socket paths from the fixed in-VM probe script only.
var podmanSocketPathRe = regexp.MustCompile(`^(/run|/var/run)/[a-zA-Z0-9/_.-]*podman\.sock$`)

type podmanMachineListEntry struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
}

func validatePodmanMachineName(name string) error {
	name = strings.TrimSpace(strings.TrimSuffix(name, "*"))
	if name == "" {
		return fmt.Errorf("empty podman machine name")
	}
	if len(name) > 128 || !podmanMachineNameRe.MatchString(name) {
		return fmt.Errorf("invalid podman machine name %q", name)
	}
	return nil
}

func validatePodmanSocketPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" || !podmanSocketPathRe.MatchString(path) {
		return fmt.Errorf("invalid podman socket path %q", path)
	}
	return nil
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
	if err := validatePodmanMachineName(machine); err != nil {
		return "", err
	}
	// Remote command is a fixed probe script (podmanSocketProbeScript); machine name is validated above.
	cmd := exec.Command("podman", "machine", "ssh", machine, podmanSocketProbeScript()) // #nosec G204 -- argv-only podman ssh; no shell; machine from podman machine list JSON
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
	if err := validatePodmanSocketPath(socketPath); err != nil {
		return "", err
	}
	return socketPath, nil
}

func podmanRunningMachineName() (string, error) {
	out, err := exec.Command("podman", "machine", "list", "--format", "json").Output() // #nosec G204 -- fixed podman argv, no user-controlled args
	if err != nil {
		return "", err
	}
	var rows []podmanMachineListEntry
	if err := json.Unmarshal(out, &rows); err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Running {
			name := strings.TrimSuffix(strings.TrimSpace(r.Name), "*")
			if err := validatePodmanMachineName(name); err != nil {
				return "", err
			}
			return name, nil
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
		cmd := exec.Command("podman", "machine", "ssh", machine, "stat", "-c", "%g", socketPath) // #nosec G204 -- validated machine and socket path; fixed stat argv
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
