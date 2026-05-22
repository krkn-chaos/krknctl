// Package randomstate manages the lifecycle state of a random chaos run.
// State is persisted as a JSON file so that status/abort commands can
// inspect or stop a running plan across process boundaries.
package randomstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const stateDirName = ".krknctl"
const stateFileName = "random_run.json"

// State holds the runtime information of an active random chaos run.
type State struct {
	PID          int       `json:"pid"`
	ScenarioName string    `json:"scenario_name"` // human-readable basename of the plan file
	PlanFile     string    `json:"plan_file"`     // full path to the plan file
	StartTime    time.Time `json:"start_time"`
	LogDir       string    `json:"log_dir,omitempty"`
}

// statePath returns the OS-appropriate path for the state file,
// using os.TempDir() so it works on Linux, macOS, and Windows.
func statePath() string {
	return filepath.Join(os.TempDir(), stateDirName, stateFileName)
}

// SaveState persists s to disk, creating the state directory if needed.
func SaveState(s *State) error {
	dir := filepath.Dir(statePath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	return os.WriteFile(statePath(), data, 0600)
}

// LoadState reads the persisted state from disk.
// Returns (nil, nil) when no state file exists — nothing is running.
func LoadState() (*State, error) {
	data, err := os.ReadFile(statePath()) // #nosec G304 -- path is constructed from os.TempDir() + fixed constants
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	var s State
	if err = json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

// ClearState removes the state file from disk. Safe to call when no state exists.
func ClearState() error {
	err := os.Remove(statePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
