package verify

import "errors"

// Sentinel errors returned by VerifyImage. Callers MUST branch on these with
// errors.Is so that the trust policy (what counts as "safe to run") is applied
// identically by every consumer of this package (krknctl CLI, krkn-operator).
//
// The concrete error returned always wraps one of these sentinels together with
// the underlying cause, so errors.Is(err, ErrUnsigned) and friends work while
// the operator/CLI can still log the low-level detail.
var (
	// ErrUnsigned means the image exists in the registry but carries no cosign
	// signature at all. The image MUST NOT be run.
	ErrUnsigned = errors.New("image is not signed")

	// ErrInvalidSignature means one or more signatures are present but none of
	// them could be verified with the trusted public key(s) — a wrong key, a
	// tampered payload, or a signature for a different digest. The image MUST
	// NOT be run.
	ErrInvalidSignature = errors.New("image signature verification failed")

	// ErrRegistryUnreachable means the signature could not be evaluated because
	// the registry could not be contacted or the manifest could not be
	// resolved (network error, auth failure, unknown repository/tag). This is
	// an operational error, not a trust decision: the caller decides whether to
	// retry, but MUST NOT run the image on this error.
	ErrRegistryUnreachable = errors.New("registry unreachable")

	// ErrInvalidReference means the supplied image reference is syntactically
	// invalid and could not be parsed.
	ErrInvalidReference = errors.New("invalid image reference")

	// ErrNoTrustedKeys means the Verifier was constructed without any usable
	// public key (embedded key failed to load and no additional keys were
	// provided). This is a build/configuration error.
	ErrNoTrustedKeys = errors.New("no trusted public keys configured")
)