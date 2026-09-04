package scenarioorchestrator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

// captureStderr redirects os.Stderr for the duration of fn and returns whatever
// was written to it. Coloring is disabled so assertions can match on the plain
// message text regardless of the ambient terminal.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	prevNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = prevNoColor }()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stderr failed: %v", err)
	}
	return buf.String()
}

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

// TestLogVerifiedImage confirms the success path emits a confirmation to stderr
// that names the image and the pinned digest that will actually run.
func TestLogVerifiedImage(t *testing.T) {
	const (
		image  = "quay.io/krkn-chaos/krkn-hub:pod-scenarios"
		digest = "quay.io/krkn-chaos/krkn-hub@sha256:0123456789abcdef"
	)

	out := captureStderr(t, func() { logVerifiedImage(image, digest) })

	for _, want := range []string{"signature verified", image, digest} {
		if !strings.Contains(out, want) {
			t.Errorf("confirmation log missing %q\ngot: %s", want, out)
		}
	}
}

// TestLogRejectedImage maps each trust-failure sentinel onto a tailored,
// human-readable rejection message on stderr.
func TestLogRejectedImage(t *testing.T) {
	const image = "quay.io/krkn-chaos/krkn-hub:pod-scenarios"

	tests := []struct {
		name       string
		err        error
		wantSubstr []string
	}{
		{
			name:       "unsigned",
			err:        fmt.Errorf("%w: %q", verify.ErrUnsigned, image),
			wantSubstr: []string{"REJECTED", "not signed", image},
		},
		{
			name:       "invalid signature",
			err:        fmt.Errorf("%w: %q", verify.ErrInvalidSignature, image),
			wantSubstr: []string{"REJECTED", "not by a trusted", image},
		},
		{
			name:       "registry unreachable",
			err:        fmt.Errorf("%w: %q", verify.ErrRegistryUnreachable, image),
			wantSubstr: []string{"REJECTED", "registry unreachable", image},
		},
		{
			name:       "invalid reference",
			err:        fmt.Errorf("%w: %q", verify.ErrInvalidReference, image),
			wantSubstr: []string{"REJECTED", "not a valid image reference", image},
		},
		{
			name:       "unknown error falls back to generic message",
			err:        errors.New("boom"),
			wantSubstr: []string{"REJECTED", "boom", image},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStderr(t, func() { logRejectedImage(image, tc.err) })
			for _, want := range tc.wantSubstr {
				if !strings.Contains(out, want) {
					t.Errorf("rejection log missing %q\ngot: %s", want, out)
				}
			}
		})
	}
}
