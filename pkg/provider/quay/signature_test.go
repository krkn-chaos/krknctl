package quay

import (
	"context"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/cache"
	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetImageSignatureStatus_CacheHit exercises the quay provider's signature
// lookup without any network access: the reference it derives from config is
// pre-seeded in the cache, so the cached status is returned verbatim. This
// covers URI construction (GetQuayImageURI + ImageReference) and the cache
// fast-path in one offline test. The registry argument is ignored by the quay
// provider (the public registry needs no credentials), so nil is passed.
func TestGetImageSignatureStatus_CacheHit(t *testing.T) {
	cfg, err := krknctlconfig.LoadConfig()
	require.NoError(t, err)
	c := cache.NewCache()
	p := ScenarioProvider{provider.BaseScenarioProvider{Config: cfg, Cache: c}}

	imageURI, err := cfg.GetQuayImageURI()
	require.NoError(t, err)

	tag := models.ScenarioTag{Name: "dummy-scenario", Digest: strptr("sha256:deadbeef")}
	ref := provider.ImageReference(imageURI, tag)

	// Seed the cache the same way BaseScenarioProvider.ImageSignatureStatus does.
	c.SetString("sigstatus:"+ref, string(verify.SignatureSigned))

	status, err := p.GetImageSignatureStatus(context.Background(), nil, tag)
	require.NoError(t, err)
	assert.Equal(t, verify.SignatureSigned, status)
}

func strptr(s string) *string { return &s }
