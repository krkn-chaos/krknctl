package registryv2

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetImageSignatureStatus_NilRegistry: this provider always requires a
// registry; a nil one is a caller error and yields unknown plus an error.
func TestGetImageSignatureStatus_NilRegistry(t *testing.T) {
	p := ScenarioProvider{
		provider.BaseScenarioProvider{Config: getConfig(t), Cache: cache.NewCache()},
	}
	status, err := p.GetImageSignatureStatus(context.Background(), nil, models.ScenarioTag{Name: "latest"})
	require.Error(t, err)
	assert.Equal(t, verify.SignatureUnknown, status)
}

// TestGetImageSignatureStatus_Unsigned verifies an image pushed (but not signed)
// to an in-memory v2 registry is reported as unsigned, exercising the full
// provider path (URI building + verify.OptionsForRegistry + verification).
func TestGetImageSignatureStatus_Unsigned(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	registryV2 := &models.RegistryV2{
		RegistryURL:        u.Host,
		ScenarioRepository: "krkn/scenario",
		Insecure:           true, // loopback in-memory registry serves plain HTTP
	}

	// Push an unsigned image at the tag the provider will resolve.
	ref, err := name.ParseReference(registryV2.GetPrivateRegistryURI() + ":dummy-scenario")
	require.NoError(t, err)
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img))

	p := ScenarioProvider{
		provider.BaseScenarioProvider{Config: getConfig(t), Cache: cache.NewCache()},
	}
	status, err := p.GetImageSignatureStatus(context.Background(), registryV2, models.ScenarioTag{Name: "dummy-scenario"})
	require.NoError(t, err)
	assert.Equal(t, verify.SignatureUnsigned, status)
}
