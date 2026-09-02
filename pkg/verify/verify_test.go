package verify

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/sigstore/cosign/v2/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRegistry starts an in-memory OCI registry on the loopback interface.
// go-containerregistry serves loopback hosts over plain HTTP, so the production
// VerifyImage code path (which parses references without any insecure flag)
// works against it unchanged.
func newTestRegistry(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host
}

// pushRandomImage pushes a fresh random image to ref and returns its digest.
func pushRandomImage(t *testing.T, ref name.Reference) name.Digest {
	t.Helper()
	img, err := random.Image(1024, 2)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img))
	h, err := img.Digest()
	require.NoError(t, err)
	return ref.Context().Digest(h.String())
}

// ephemeralKey generates an ECDSA P-256 signer and its PEM-encoded public key.
func ephemeralKey(t *testing.T) (signature.SignerVerifier, []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	sv, err := signature.LoadECDSASignerVerifier(priv, crypto.SHA256)
	require.NoError(t, err)
	pubPEM, err := cryptoutils.MarshalPublicKeyToPEM(sv.Public())
	require.NoError(t, err)
	return sv, pubPEM
}

// signImage signs digest with sv and writes the cosign signature artifact next
// to the image in the registry, exactly as `cosign sign` would.
func signImage(t *testing.T, digest name.Digest, sv signature.SignerVerifier) {
	t.Helper()

	payloadBytes, err := (&payload.Cosign{Image: digest}).MarshalJSON()
	require.NoError(t, err)

	sig, err := sv.SignMessage(bytes.NewReader(payloadBytes))
	require.NoError(t, err)
	b64sig := base64.StdEncoding.EncodeToString(sig)

	ociSig, err := static.NewSignature(payloadBytes, b64sig)
	require.NoError(t, err)

	se, err := ociremote.SignedEntity(digest)
	require.NoError(t, err)
	newSE, err := mutate.AttachSignatureToEntity(se, ociSig)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteSignatures(digest.Repository, newSE))
}

func TestVerifyImage_Signed(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:latest")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	sv, pubPEM := ephemeralKey(t)
	signImage(t, digest, sv)

	res, err := VerifyImage(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	require.NoError(t, err)
	assert.Equal(t, ref.Name(), res.Reference)
	assert.Equal(t, digest.DigestStr(), res.DigestString)
	assert.Contains(t, res.Digest, "@sha256:")
	// The returned digest must pin the exact image we pushed.
	assert.Equal(t, digest.Name(), res.Digest)
}

func TestVerifyImage_Unsigned(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:unsigned")
	require.NoError(t, err)
	_ = pushRandomImage(t, ref)

	sv, pubPEM := ephemeralKey(t)
	_ = sv // key present but the image was never signed

	_, err = VerifyImage(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsigned), "expected ErrUnsigned, got %v", err)
}

func TestVerifyImage_WrongKey(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:wrongkey")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	// Sign with one key...
	signer, _ := ephemeralKey(t)
	signImage(t, digest, signer)

	// ...but verify while trusting only a different key (plus the embedded one).
	_, otherPubPEM := ephemeralKey(t)
	_, err = VerifyImage(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{otherPubPEM},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSignature), "expected ErrInvalidSignature, got %v", err)
}

func TestVerifyImage_Tampered(t *testing.T) {
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:tampered")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	// Attach a signature whose payload references the image but whose signature
	// bytes are garbage: signatures are present, but none verify.
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
	_, err = VerifyImage(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{pubPEM},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSignature), "expected ErrInvalidSignature, got %v", err)
}

func TestVerifyImage_RegistryUnreachable(t *testing.T) {
	// Port 1 is not listening; resolving the digest must fail.
	_, err := VerifyImage(context.Background(), "127.0.0.1:1/krkn/scenario:latest", Options{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRegistryUnreachable), "expected ErrRegistryUnreachable, got %v", err)
}

func TestVerifyImage_InvalidReference(t *testing.T) {
	_, err := VerifyImage(context.Background(), "NOT A VALID REF", Options{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidReference), "expected ErrInvalidReference, got %v", err)
}

func TestVerifyImage_MissingImage(t *testing.T) {
	host := newTestRegistry(t)
	// Nothing was pushed to this repo.
	_, err := VerifyImage(context.Background(), host+"/krkn/absent:latest", Options{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRegistryUnreachable), "expected ErrRegistryUnreachable, got %v", err)
}

func TestNew_EmbeddedKeyLoads(t *testing.T) {
	// The embedded key alone must be enough to build a verifier.
	v, err := New(Options{})
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Len(t, v.verifiers, 1)
}

func TestNew_AdditionalKeyAppended(t *testing.T) {
	_, pubPEM := ephemeralKey(t)
	v, err := New(Options{AdditionalPublicKeys: [][]byte{pubPEM}})
	require.NoError(t, err)
	assert.Len(t, v.verifiers, 2, "embedded key + one additional key")
}

func TestNew_InvalidAdditionalKey(t *testing.T) {
	_, err := New(Options{AdditionalPublicKeys: [][]byte{[]byte("-----BEGIN PUBLIC KEY-----\ngarbage\n-----END PUBLIC KEY-----")}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoTrustedKeys), "expected ErrNoTrustedKeys, got %v", err)
}

func TestVerifyImage_MultiKeyRotation(t *testing.T) {
	// Simulate rotation: image signed with the "new" key, verifier trusts both
	// the embedded (old) key and the new key.
	host := newTestRegistry(t)
	ref, err := name.ParseReference(host + "/krkn/scenario:rotate")
	require.NoError(t, err)
	digest := pushRandomImage(t, ref)

	newSigner, newPubPEM := ephemeralKey(t)
	signImage(t, digest, newSigner)

	res, err := VerifyImage(context.Background(), ref.Name(), Options{
		AdditionalPublicKeys: [][]byte{newPubPEM},
	})
	require.NoError(t, err)
	assert.Equal(t, digest.Name(), res.Digest)
}

// Guard: the embedded public key must be a valid PEM public key at all times.
func TestEmbeddedPublicKey_Valid(t *testing.T) {
	assert.True(t, strings.Contains(string(embeddedPublicKey), "BEGIN PUBLIC KEY"))
	_, err := cryptoutils.UnmarshalPEMToPublicKey(embeddedPublicKey)
	require.NoError(t, err)
}
