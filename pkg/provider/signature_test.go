package provider

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sptr(s string) *string { return &s }

// TestImageReference covers digest pinning versus tag fallback.
func TestImageReference(t *testing.T) {
	cases := []struct {
		name string
		base string
		tag  models.ScenarioTag
		want string
	}{
		{
			name: "digest present pins by digest",
			base: "quay.io/krkn-chaos/krkn-hub",
			tag:  models.ScenarioTag{Name: "latest", Digest: sptr("sha256:abc")},
			want: "quay.io/krkn-chaos/krkn-hub@sha256:abc",
		},
		{
			name: "nil digest falls back to tag",
			base: "quay.io/krkn-chaos/krkn-hub",
			tag:  models.ScenarioTag{Name: "v1.2.3"},
			want: "quay.io/krkn-chaos/krkn-hub:v1.2.3",
		},
		{
			name: "empty digest falls back to tag",
			base: "quay.io/krkn-chaos/krkn-hub",
			tag:  models.ScenarioTag{Name: "v1.2.3", Digest: sptr("")},
			want: "quay.io/krkn-chaos/krkn-hub:v1.2.3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ImageReference(tc.base, tc.tag))
		})
	}
}

// startTestRegistry starts an in-memory OCI registry on loopback (served over
// plain HTTP by go-containerregistry) and returns its host and a stop func.
func startTestRegistry(t *testing.T) (host string, stop func()) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host, srv.Close
}

func newBaseProvider(t *testing.T) BaseScenarioProvider {
	t.Helper()
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	return BaseScenarioProvider{Config: cfg, Cache: cache.NewCache()}
}

// TestImageSignatureStatus_CachesDefinitiveResult proves a definitive status is
// cached: after the first lookup returns "unsigned" for a pushed-but-unsigned
// image, the registry is shut down; a second lookup that still returns
// "unsigned" (rather than "unknown") can only have come from the cache.
func TestImageSignatureStatus_CachesDefinitiveResult(t *testing.T) {
	host, stop := startTestRegistry(t)

	ref, err := name.ParseReference(host + "/krkn/scenario:unsigned")
	require.NoError(t, err)
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img))

	p := newBaseProvider(t)

	first := p.ImageSignatureStatus(context.Background(), ref.Name(), verify.Options{})
	assert.Equal(t, verify.SignatureUnsigned, first)

	// Kill the registry: any further network call would now yield "unknown".
	stop()

	second := p.ImageSignatureStatus(context.Background(), ref.Name(), verify.Options{})
	assert.Equal(t, verify.SignatureUnsigned, second, "definitive result must be served from cache")
}

// TestImageSignatureStatus_DoesNotCacheUnknown proves transient "unknown"
// results are never cached: a reachable, definitive result on a later call must
// override the earlier outage rather than being masked by a cached "unknown".
func TestImageSignatureStatus_DoesNotCacheUnknown(t *testing.T) {
	host, stop := startTestRegistry(t)
	ref := host + "/krkn/scenario:later"

	p := newBaseProvider(t)

	// Registry is up but nothing is pushed yet: the image is absent, which the
	// verifier reports as an operational failure -> unknown.
	first := p.ImageSignatureStatus(context.Background(), ref, verify.Options{})
	assert.Equal(t, verify.SignatureUnknown, first)

	// Now push the image; if "unknown" had been cached this would still be
	// unknown. It must re-evaluate to a definitive "unsigned".
	parsed, err := name.ParseReference(ref)
	require.NoError(t, err)
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(parsed, img))
	t.Cleanup(stop)

	second := p.ImageSignatureStatus(context.Background(), ref, verify.Options{})
	assert.Equal(t, verify.SignatureUnsigned, second, "unknown must not be cached")
}
