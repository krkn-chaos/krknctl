package verify

import (
	"context"
	"crypto"
	_ "embed"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	cosignsig "github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
)

// embeddedPublicKey is the ecosystem cosign public key. It is committed
// alongside this package and embedded at build time so that every consumer
// (krknctl CLI, krkn-operator) trusts exactly the same key. The matching
// private key is a CI secret used by krkn CI to sign the base image and the
// krkn-hub scenario images (`cosign sign --key env://COSIGN_PRIVATE_KEY
// --tlog-upload=false <repo>@sha256:...`), signing the digest, not the tag.
//
// WARNING: the currently committed cosign.pub is a PLACEHOLDER generated
// locally so the package builds and tests pass. It MUST be replaced with the
// real ecosystem public key produced by `cosign generate-key-pair` when the CI
// signing key pair is created (krkn "sign images in CI" issue).
//
// Rotation (picked up by both consumers on their next release):
//  1. Generate a new key pair.
//  2. During the transition, keep trusting the old key by passing it via
//     Options.AdditionalPublicKeys while images are re-signed with the new one.
//  3. Replace cosign.pub with the new public key and update the CI secret.
//  4. Once every image is re-signed and released, drop the old key.
//
//go:embed cosign.pub
var embeddedPublicKey []byte

// Options configures signature verification policy. The zero value is the
// secure, offline default used across the krkn ecosystem: key-based
// verification against the embedded public key, no transparency-log lookup,
// registry auth from the ambient keychain.
type Options struct {
	// AdditionalPublicKeys are extra PEM-encoded public keys accepted in
	// addition to the embedded ecosystem key. An image is considered verified
	// if it is signed by ANY trusted key. Primarily used to support key
	// rotation (trust old + new during a transition window) and to inject an
	// ephemeral key in tests. Callers on the runtime path normally leave this
	// nil and rely solely on the embedded key.
	AdditionalPublicKeys [][]byte

	// Keychain overrides how registry credentials are resolved. When nil,
	// authn.DefaultKeychain is used (docker config / ambient credentials).
	Keychain authn.Keychain

	// RemoteOptions are extra go-containerregistry remote options appended to
	// the defaults (e.g. a custom transport for a private/insecure registry or
	// tests). The request context is always injected regardless of this slice.
	RemoteOptions []remote.Option

	// RequireTlog, when true, enforces Rekor transparency-log verification.
	// Default (false) performs offline key-based verification only, which is
	// the ecosystem policy (no Fulcio/Rekor dependency, air-gap friendly).
	RequireTlog bool
}

// VerifiedImage is the result of a successful verification. Callers MUST run the
// pinned Digest, never the tag they passed in, to avoid a time-of-check /
// time-of-use gap between verification and execution.
type VerifiedImage struct {
	// Reference is the original reference passed to VerifyImage.
	Reference string
	// Digest is the fully-qualified digest reference (repo@sha256:...). Run this.
	Digest string
	// DigestString is just the algorithm:hex portion (sha256:...).
	DigestString string
}

// Verifier holds the trusted keys and registry options for repeated
// verifications. It is safe to construct once and reuse for many images.
type Verifier struct {
	verifiers   []signature.Verifier
	keychain    authn.Keychain
	remoteOpts  []remote.Option
	requireTlog bool
}

// New builds a Verifier from the embedded ecosystem public key plus any
// additional keys supplied in opts. It returns ErrNoTrustedKeys if no key could
// be loaded.
func New(opts Options) (*Verifier, error) {
	var verifiers []signature.Verifier

	// The embedded key is the canonical ecosystem key. A failure to load it is
	// a build error, but we tolerate it as long as at least one additional key
	// is usable (e.g. tests that inject their own key), and fail closed only if
	// no key at all could be loaded.
	if v, err := cosignsig.LoadPublicKeyRaw(embeddedPublicKey, crypto.SHA256); err == nil {
		verifiers = append(verifiers, v)
	}

	for _, pem := range opts.AdditionalPublicKeys {
		v, err := cosignsig.LoadPublicKeyRaw(pem, crypto.SHA256)
		if err != nil {
			return nil, fmt.Errorf("%w: loading additional public key: %v", ErrNoTrustedKeys, err)
		}
		verifiers = append(verifiers, v)
	}

	if len(verifiers) == 0 {
		return nil, ErrNoTrustedKeys
	}

	keychain := opts.Keychain
	if keychain == nil {
		keychain = authn.DefaultKeychain
	}

	return &Verifier{
		verifiers:   verifiers,
		keychain:    keychain,
		remoteOpts:  opts.RemoteOptions,
		requireTlog: opts.RequireTlog,
	}, nil
}

// VerifyImage is a convenience wrapper that builds a Verifier from opts and
// verifies a single reference. For repeated verifications, build a Verifier once
// with New and call Verifier.VerifyImage.
func VerifyImage(ctx context.Context, ref string, opts Options) (VerifiedImage, error) {
	v, err := New(opts)
	if err != nil {
		return VerifiedImage{}, err
	}
	return v.VerifyImage(ctx, ref)
}

// VerifyImage resolves ref to a digest and verifies its cosign signature
// against the trusted keys. On success it returns the pinned digest, which the
// caller MUST use to run the image. On failure it returns one of the sentinel
// errors (ErrUnsigned, ErrInvalidSignature, ErrRegistryUnreachable,
// ErrInvalidReference) wrapped with the underlying cause; test with errors.Is.
func (v *Verifier) VerifyImage(ctx context.Context, ref string) (VerifiedImage, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return VerifiedImage{}, fmt.Errorf("%w: %q: %v", ErrInvalidReference, ref, err)
	}

	ociOpts := v.ociRemoteOptions(ctx)

	// Resolve tag -> digest first. This both pins the digest (anti-TOCTOU) and
	// lets us distinguish a registry/manifest problem from a signature problem.
	digest, err := ociremote.ResolveDigest(parsed, ociOpts...)
	if err != nil {
		return VerifiedImage{}, fmt.Errorf("%w: resolving %q: %v", ErrRegistryUnreachable, ref, err)
	}

	co := &cosign.CheckOpts{
		SigVerifier:        nil, // set per-key below
		IgnoreTlog:         !v.requireTlog,
		IgnoreSCT:          true,
		Offline:            !v.requireTlog,
		RegistryClientOpts: ociOpts,
	}

	// An image is trusted if ANY configured key verifies it. We verify the
	// pinned digest, not the tag, so the signature is bound to the exact bytes
	// we resolved above.
	var sawSignatures bool
	var lastErr error
	for _, sv := range v.verifiers {
		co.SigVerifier = sv
		_, _, verr := cosign.VerifyImageSignatures(ctx, digest, co)
		if verr == nil {
			return VerifiedImage{
				Reference:    ref,
				Digest:       digest.Name(),
				DigestString: digest.DigestStr(),
			}, nil
		}
		// A "no matching signatures" error means signatures exist but this key
		// did not validate them — remember that so we report invalid-signature
		// rather than unsigned if no key ends up matching.
		var noMatch *cosign.ErrNoMatchingSignatures
		if errors.As(verr, &noMatch) {
			sawSignatures = true
		}
		lastErr = verr
	}

	return VerifiedImage{}, classifyVerifyError(ref, sawSignatures, lastErr)
}

// ociRemoteOptions builds the cosign ociremote options, always injecting the
// request context and the configured keychain, then any caller-supplied remote
// options.
func (v *Verifier) ociRemoteOptions(ctx context.Context) []ociremote.Option {
	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(v.keychain),
	}
	remoteOpts = append(remoteOpts, v.remoteOpts...)
	return []ociremote.Option{ociremote.WithRemoteOptions(remoteOpts...)}
}

// classifyVerifyError maps a cosign verification error onto one of the package
// sentinels. sawSignatures is true when at least one key reported that
// signatures were present but did not match (i.e. the image is signed, just not
// by us).
func classifyVerifyError(ref string, sawSignatures bool, err error) error {
	if err == nil {
		// Defensive: no key succeeded yet no error was recorded.
		return fmt.Errorf("%w: %q: no key verified the signature", ErrInvalidSignature, ref)
	}

	// Signatures are entirely absent -> the image is unsigned.
	var noSigs *cosign.ErrNoSignaturesFound
	if errors.As(err, &noSigs) && !sawSignatures {
		return fmt.Errorf("%w: %q: %v", ErrUnsigned, ref, err)
	}

	// Signatures present but none matched our keys -> invalid signature.
	var noMatch *cosign.ErrNoMatchingSignatures
	if sawSignatures || errors.As(err, &noMatch) {
		return fmt.Errorf("%w: %q: %v", ErrInvalidSignature, ref, err)
	}

	// The signature artifact tag itself could not be fetched -> registry issue.
	var tagNotFound *cosign.ErrImageTagNotFound
	if errors.As(err, &tagNotFound) {
		return fmt.Errorf("%w: fetching signatures for %q: %v", ErrRegistryUnreachable, ref, err)
	}

	// Anything else (bad payload, crypto failure, malformed signature) is a
	// verification failure: fail closed rather than treating it as unsigned.
	return fmt.Errorf("%w: %q: %v", ErrInvalidSignature, ref, err)
}