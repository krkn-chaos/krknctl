package scenarioorchestratortest

import (
	"errors"
	"testing"
)

func TestIsRejectedQuayToken(t *testing.T) {
	if !isRejectedQuayToken(errors.New(`unauthorized: {"error": "Invalid bearer token format"}`)) {
		t.Fatal("invalid bearer token response was not recognized")
	}
	if isRejectedQuayToken(errors.New("container runtime unavailable")) {
		t.Fatal("unrelated runtime error was recognized as a rejected token")
	}
	if isRejectedQuayToken(nil) {
		t.Fatal("nil error was recognized as a rejected token")
	}
}
