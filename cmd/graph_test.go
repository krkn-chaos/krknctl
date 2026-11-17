package cmd

import (
	"errors"
	"testing"
)

func TestGraphExecutionErrorHonorsExitOnError(t *testing.T) {
	cause := errors.New("trigger verification failed")

	if err := graphExecutionError(cause, false); err != nil {
		t.Fatalf("graphExecutionError with exit-on-error disabled = %v, want nil", err)
	}
	if err := graphExecutionError(cause, true); !errors.Is(err, cause) {
		t.Fatalf("graphExecutionError with exit-on-error enabled = %v, want wrapped cause", err)
	}
}

func TestGraphExecutionErrorIgnoresNilError(t *testing.T) {
	if err := graphExecutionError(nil, true); err != nil {
		t.Fatalf("graphExecutionError with nil cause = %v, want nil", err)
	}
}
