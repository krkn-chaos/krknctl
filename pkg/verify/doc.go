// Package verify provides the single source of truth for cosign image
// signature verification across the krkn ecosystem.
//
// # Why this package exists
//
// The krkn-operator and krknctl both execute third-party scenario images
// (quay.io/krkn-chaos/krkn-hub:<tag>, private registries, the graph-run
// "complete image"). Running an image we do not control is the executor's
// responsibility, so only images we have verified may run. If krknctl verified
// with one logic and the operator with another, an attacker would exploit the
// weaker path. These primitives are therefore written once here and imported by
// both consumers so the trust policy is byte-for-byte identical everywhere.
//
// # Trust model
//
// krknctl and krkn-operator are trusted by construction (we build and push
// them, from repositories under our control). They are the trust anchor and do
// not verify themselves; they VERIFY the scenario images they execute. A
// verified scenario image is considered trustworthy at the same level as the
// executor.
//
// # Design
//
//   - Key-based cosign (NOT keyless): no Fulcio/Rekor dependency, fully offline.
//     The private key lives as a CI secret (krkn CI signs at build time); the
//     public key is committed and embedded here via go:embed, so both key and
//     logic have a single source of truth. Rotation = update the embedded key
//     (optionally keeping the old one via Options.AdditionalPublicKeys during a
//     transition) and both consumers pick it up on their next release.
//   - Verification pins the DIGEST, not the tag (anti-TOCTOU): VerifyImage
//     resolves tag -> digest and returns the digest; callers MUST run the
//     returned digest, never the original tag.
//   - Signing happens with the cosign CLI in CI, so this package is
//     verify-only: it depends on cosign-lib + stdlib and MUST NOT import CLI
//     code or operator CRD types (avoids an import cycle and keeps the CLI out
//     of the operator binary).
//
// # Air-gap / mirroring
//
// cosign stores the signature as a separate artifact (sha256-<digest>.sig) in
// the same repository. Generic mirror tools (skopeo copy, oc image mirror,
// docker pull/push) copy only the image and leave the signature behind. In
// air-gapped environments operators must mirror with `cosign copy` so the image
// and its signature travel together; verification then works offline against
// the mirrored registry. This is an operational/documentation concern with no
// code impact here.
package verify