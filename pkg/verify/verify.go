package verify

import (
	"context"
	"crypto"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	cosignsig "github.com/sigstore/cosign/v3/pkg/signature"
	sgverify "github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
)

// embeddedPublicKey is the ecosystem cosign public key. It is committed
// alongside this package and embedded at build time so that every consumer
// (krknctl CLI, krkn-operator) trusts exactly the same key. The matching
// private key is a CI secret used by krkn CI to sign the base image and the
// krkn-hub scenario images (`cosign sign --key env://COSIGN_PRIVATE_KEY
// --tlog-upload=false <repo>@sha256:...`), signing the digest, not the tag.
//
// cosign.pub is the real ecosystem public key produced by
// `cosign generate-key-pair`; its private half is the COSIGN_PRIVATE_KEY CI
// secret used to sign the krkn base image (krkn-chaos/krkn) and the krkn-hub
// scenario images (krkn-chaos/krkn-hub and the release-time rebuild in
// redhat-chaos/actions).
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

	// Insecure allows contacting the registry over plain HTTP instead of HTTPS.
	// Use only for private/air-gapped registries served without TLS. Default
	// false (HTTPS). This is orthogonal to skipping TLS certificate
	// verification, which callers configure via a custom transport in
	// RemoteOptions.
	Insecure bool
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
	verifiers  []signature.Verifier
	keychain   authn.Keychain
	remoteOpts []remote.Option
	insecure   bool
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
		verifiers:  verifiers,
		keychain:   keychain,
		remoteOpts: opts.RemoteOptions,
		insecure:   opts.Insecure,
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
	var nameOpts []name.Option
	if v.insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	parsed, err := name.ParseReference(ref, nameOpts...)
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
		SigVerifier: nil, // set per-key below
		// The ecosystem signs with `--tlog-upload=false`, so no transparency-log
		// entry ever exists: verification is strictly offline and key-based
		// (no Fulcio/Rekor dependency, air-gap friendly).
		IgnoreTlog: true,
		IgnoreSCT:  true,
		Offline:    true,
		// Discover signatures stored either as a legacy digest signature tag or
		// as an OCI 1.1 referrer; cosign tries referrers first, then falls back.
		ExperimentalOCI11:  true,
		RegistryClientOpts: ociOpts,
	}

	// Prefer the new sigstore bundle format (an OCI 1.1 referrer carrying a DSSE
	// envelope), which cosign 2.6+/3.x emit by default and which the krkn-hub
	// images now use. If such a bundle is attached it is authoritative: a
	// definitive verified/untrusted result is returned and we never fall back to
	// the legacy path. Only when no new-format bundle exists do we drop through
	// to the legacy .sig/referrer verification below (for older images and the
	// base image, which may still carry the legacy format).
	if vi, verified, nberr := v.verifyNewBundleFormat(ctx, ref, digest, co, nameOpts); nberr != nil {
		return VerifiedImage{}, nberr
	} else if verified {
		return vi, nil
	}

	// An image is trusted if ANY configured key verifies it. We verify the
	// pinned digest, not the tag, so the signature is bound to the exact bytes
	// we resolved above.
	var sawSignatures bool
	var lastErr error
	var registryErr error
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
		// A transport/network failure fetching the signature artifact is an
		// operational error, not a trust decision. Record it and stop: retrying
		// other keys re-hits the same unreachable registry, and we must not let a
		// later key's verification result mask a registry outage.
		if isRegistryError(verr) {
			registryErr = verr
			break
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

	if registryErr != nil {
		return VerifiedImage{}, fmt.Errorf("%w: fetching signatures for %q: %v", ErrRegistryUnreachable, ref, registryErr)
	}

	return VerifiedImage{}, classifyVerifyError(ref, sawSignatures, lastErr)
}

// verifyNewBundleFormat attempts verification using the new sigstore bundle
// format (OCI 1.1 referrers carrying a DSSE envelope), which cosign 2.6+/3.x
// produce by default. The vendored cosign library refuses to verify these via
// the classic VerifyImageSignatures path ("bundle support for image signatures
// is not yet implemented"), so we mirror what the cosign v3 CLI does: enumerate
// the attached bundles with GetBundles and verify each with VerifyNewBundle,
// binding the DSSE payload to the pinned image digest.
//
// It returns:
//
//   - (vi, true, nil)  when a trusted key verifies an attached bundle;
//   - (_, false, err)  when the image carries new-format bundles but none is
//     trusted, or the registry could not be reached — a definitive result the
//     caller must return verbatim;
//   - (_, false, nil)  when no new-format bundle is attached, signalling the
//     caller to fall back to the legacy verification path.
//
// The bundle verification uses key-based offline trust: with co.SigVerifier set
// and IgnoreTlog true, cosign builds the trusted material from the public key
// alone, so no Fulcio/Rekor/TUF material is required (air-gap friendly).
func (v *Verifier) verifyNewBundleFormat(ctx context.Context, ref string, digest name.Digest, co *cosign.CheckOpts, nameOpts []name.Option) (VerifiedImage, bool, error) {
	bundles, hash, err := cosign.GetBundles(ctx, digest, co.RegistryClientOpts, nameOpts...)
	if err != nil {
		// An operational registry/transport failure is not a trust decision;
		// surface it so a later key can't mask an outage.
		if isRegistryError(err) {
			return VerifiedImage{}, false, fmt.Errorf("%w: fetching signature bundles for %q: %v", ErrRegistryUnreachable, ref, err)
		}
		// No new-format bundles present (ErrNoMatchingAttestations), a registry
		// without referrers support (404), or any other non-operational
		// condition: let the caller try the legacy path.
		return VerifiedImage{}, false, nil
	}
	if len(bundles) == 0 {
		return VerifiedImage{}, false, nil
	}

	digestBytes, err := hex.DecodeString(hash.Hex)
	if err != nil {
		return VerifiedImage{}, false, fmt.Errorf("%w: %q: decoding digest: %v", ErrInvalidReference, ref, err)
	}
	artifactPolicy := sgverify.WithArtifactDigest(hash.Algorithm, digestBytes)

	// An image is trusted if ANY configured key verifies ANY attached bundle.
	// Work on a copy so the SigVerifier we set here never leaks into the legacy
	// loop's CheckOpts.
	coCopy := *co
	coCopy.NewBundleFormat = true
	for _, sv := range v.verifiers {
		coCopy.SigVerifier = sv
		for _, b := range bundles {
			if _, verr := cosign.VerifyNewBundle(ctx, &coCopy, artifactPolicy, b); verr == nil {
				return VerifiedImage{
					Reference:    ref,
					Digest:       digest.Name(),
					DigestString: digest.DigestStr(),
				}, true, nil
			}
		}
	}

	// New-format bundles are attached but none is signed by a trusted key: the
	// image is signed, just not by us. This is definitive — fail closed rather
	// than falling through to the legacy path and reporting "unsigned".
	return VerifiedImage{}, false, fmt.Errorf("%w: %q: no trusted key verified the attached signature bundle", ErrInvalidSignature, ref)
}

// ociRemoteOptions builds the cosign ociremote options. It applies the
// configured keychain and any caller-supplied remote options first, then injects
// the request context LAST so it always wins: go-containerregistry applies
// functional options in order and remote.WithContext assigns the context
// directly, so a caller-provided WithContext must not be able to replace the
// run's cancellation/deadline.
func (v *Verifier) ociRemoteOptions(ctx context.Context) []ociremote.Option {
	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(v.keychain),
	}
	remoteOpts = append(remoteOpts, v.remoteOpts...)
	remoteOpts = append(remoteOpts, remote.WithContext(ctx))
	return []ociremote.Option{ociremote.WithRemoteOptions(remoteOpts...)}
}

// isRegistryError reports whether err is an operational registry/transport
// failure (network error, context cancellation/deadline, or a non-404 HTTP
// status such as 401/403/5xx) rather than a cryptographic trust failure. A 404
// is deliberately excluded: a missing signature tag means the image is unsigned,
// which cosign already surfaces through its own sentinels.
func isRegistryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode != http.StatusNotFound
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return true
	}
	var nerr net.Error
	return errors.As(err, &nerr)
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
