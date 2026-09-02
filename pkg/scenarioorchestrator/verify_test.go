package scenarioorchestrator

import (
	"context"
	"testing"
)

// TestVerifyAndPinImageOrBypass_BypassReturnsImageUnchanged verifies that the
// --run-unsigned-images escape hatch skips verification entirely: it returns the
// original reference unchanged, never errors, and never touches the network
// (an obviously invalid reference would fail if verification were attempted).
func TestVerifyAndPinImageOrBypass_BypassReturnsImageUnchanged(t *testing.T) {
	const image = "quay.io/krkn-chaos/krkn-hub:pod-scenarios"

	got, err := VerifyAndPinImageOrBypass(context.Background(), image, nil, true)
	if err != nil {
		t.Fatalf("bypass must not return an error, got %v", err)
	}
	if got != image {
		t.Fatalf("bypass must return the original image unchanged: want %q, got %q", image, got)
	}
}

// TestVerifyAndPinImageOrBypass_BypassSkipsInvalidReference proves the bypass is
// a true short-circuit: even a reference that would fail to parse during
// verification is returned as-is, confirming no verification path runs.
func TestVerifyAndPinImageOrBypass_BypassSkipsInvalidReference(t *testing.T) {
	const invalid = "not a valid @@@ reference"

	got, err := VerifyAndPinImageOrBypass(context.Background(), invalid, nil, true)
	if err != nil {
		t.Fatalf("bypass must not attempt to parse/verify the reference, got %v", err)
	}
	if got != invalid {
		t.Fatalf("bypass must return the input unchanged: want %q, got %q", invalid, got)
	}
}

// TestVerifyAndPinImageOrBypass_NoBypassStillVerifies confirms that with the
// flag off the function delegates to verification: an invalid reference is
// rejected (fail-closed) rather than returned. This uses an unparseable
// reference so the check fails before any network access.
func TestVerifyAndPinImageOrBypass_NoBypassStillVerifies(t *testing.T) {
	const invalid = "not a valid @@@ reference"

	got, err := VerifyAndPinImageOrBypass(context.Background(), invalid, nil, false)
	if err == nil {
		t.Fatalf("with bypass disabled an invalid reference must be rejected, got image %q", got)
	}
	if got != "" {
		t.Fatalf("a verification failure must not return an image, got %q", got)
	}
}
