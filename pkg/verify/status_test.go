package verify

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusFor_Signed maps a clean, trusted signature onto SignatureSigned.
func TestStatusFor_Signed(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:signed")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	sv, pubPEM := ephemeralKey(t)
	signImage(t, digest, sv)

	status := StatusFor(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	assert.Equal(t, SignatureSigned, status)
}

// TestStatusFor_Unsigned maps ErrUnsigned onto SignatureUnsigned.
func TestStatusFor_Unsigned(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:unsigned")
	require.NoError(t, err)
	_ = pushRandomImage(t, ref)

	_, pubPEM := ephemeralKey(t)
	status := StatusFor(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	assert.Equal(t, SignatureUnsigned, status)
}

// TestStatusFor_Untrusted maps ErrInvalidSignature (signature present but no
// trusted key verifies it) onto SignatureUntrusted.
func TestStatusFor_Untrusted(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:wrongkey")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	// Sign with one key, trust only a different one.
	signer, _ := ephemeralKey(t)
	signImage(t, digest, signer)
	_, otherPubPEM := ephemeralKey(t)

	status := StatusFor(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{otherPubPEM},
	})
	assert.Equal(t, SignatureUntrusted, status)
}

// TestStatusFor_TamperedUntrusted maps a present-but-unverifiable signature
// (garbage signature bytes) onto SignatureUntrusted.
func TestStatusFor_TamperedUntrusted(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:tampered")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	payloadBytes, err := (&payload.Cosign{Image: digest}).MarshalJSON()
	require.NoError(t, err)
	ociSig, err := static.NewSignature(payloadBytes, base64.StdEncoding.EncodeToString([]byte("not-a-real-signature")))
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(digest)
	require.NoError(t, err)
	newSE, err := mutate.AttachSignatureToEntity(se, ociSig)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteSignatures(digest.Repository, newSE))

	_, pubPEM := ephemeralKey(t)
	status := StatusFor(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	assert.Equal(t, SignatureUntrusted, status)
}

// TestStatusFor_UnreachableUnknown maps ErrRegistryUnreachable (an operational
// failure, not a trust decision) onto SignatureUnknown.
func TestStatusFor_UnreachableUnknown(t *testing.T) {
	status := StatusFor(context.Background(), "127.0.0.1:1/krkn/scenario:latest", Options{})
	assert.Equal(t, SignatureUnknown, status)
}

// TestStatusFor_InvalidReferenceUnknown maps ErrInvalidReference onto
// SignatureUnknown (a malformed reference is not a trust decision).
func TestStatusFor_InvalidReferenceUnknown(t *testing.T) {
	status := StatusFor(context.Background(), "NOT A VALID REF", Options{})
	assert.Equal(t, SignatureUnknown, status)
}

// TestStatusFor_CancelledContextUnknown proves a cancelled context yields
// unknown (the run was aborted, not a signature verdict) even for an image that
// is present and correctly signed.
func TestStatusFor_CancelledContextUnknown(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:ctx")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	sv, pubPEM := ephemeralKey(t)
	signImage(t, digest, sv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status := StatusFor(ctx, ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	assert.Equal(t, SignatureUnknown, status)
}
