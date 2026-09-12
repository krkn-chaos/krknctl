package verify

import (
	"context"
	"errors"
)

// SignatureStatus is a coarse, presentation-friendly summary of an image's
// cosign signature state. It exists so consumers (the krknctl CLI listing, and
// later the krkn-operator API/frontend) can react to whether a scenario image
// is signed without having to branch on the low-level sentinel errors returned
// by VerifyImage themselves.
//
// The zero value is deliberately not a valid status: callers obtain a status
// exclusively through StatusFor so the mapping (and its fail-safe default)
// stays in one place.
type SignatureStatus string

const (
	// SignatureSigned means the image carries a cosign signature that verifies
	// against a trusted ecosystem key. It is safe to run.
	SignatureSigned SignatureStatus = "signed"

	// SignatureUnsigned means the image exists but carries no cosign signature
	// at all.
	SignatureUnsigned SignatureStatus = "unsigned"

	// SignatureUntrusted means one or more signatures are present but none
	// verify against a trusted key (wrong key, tampered payload, or a signature
	// for a different digest).
	SignatureUntrusted SignatureStatus = "untrusted"

	// SignatureUnknown means the signature state could not be determined. This
	// is NOT a trust decision: it covers operational failures (registry
	// unreachable, timeout), an unparseable reference, a misconfigured verifier,
	// or any unexpected error. It is transient by nature and must not be cached
	// as if it were a definitive result.
	SignatureUnknown SignatureStatus = "unknown"
)

// StatusFor verifies ref with the given options and maps the outcome onto a
// SignatureStatus. It never returns an error: every failure is folded into a
// status, and the mapping fails safe — anything that is not a clean, trusted
// signature is reported as unsigned, untrusted, or unknown, never signed.
//
// This is the single place the ecosystem's error-to-status policy is defined,
// so every consumer classifies signature state identically.
func StatusFor(ctx context.Context, ref string, opts Options) SignatureStatus {
	_, err := VerifyImage(ctx, ref, opts)
	switch {
	case err == nil:
		return SignatureSigned
	case errors.Is(err, ErrUnsigned):
		return SignatureUnsigned
	case errors.Is(err, ErrInvalidSignature):
		return SignatureUntrusted
	default:
		// ErrRegistryUnreachable, ErrInvalidReference, ErrNoTrustedKeys, or any
		// other unexpected error: not a definitive trust decision.
		return SignatureUnknown
	}
}
