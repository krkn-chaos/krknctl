package verify

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strptr(s string) *string { return &s }

// testResource is a minimal authn.Resource for exercising keychain resolution.
type testResource struct{}

func (testResource) String() string      { return "example.com" }
func (testResource) RegistryStr() string { return "example.com" }

// TestOptionsForRegistry_Nil: a nil registry (the default public registry)
// yields the zero Options — no custom keychain, no custom transport, HTTPS.
func TestOptionsForRegistry_Nil(t *testing.T) {
	opts := OptionsForRegistry(nil)
	assert.Nil(t, opts.Keychain)
	assert.False(t, opts.Insecure)
	assert.Empty(t, opts.RemoteOptions)
}

// TestOptionsForRegistry_Token: a bearer token becomes a static keychain that
// resolves to an authn.Bearer authenticator.
func TestOptionsForRegistry_Token(t *testing.T) {
	reg := &providermodels.RegistryV2{Token: strptr("jwt-abc")}
	opts := OptionsForRegistry(reg)

	require.NotNil(t, opts.Keychain)
	auth, err := opts.Keychain.Resolve(testResource{})
	require.NoError(t, err)
	cfg, err := auth.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "jwt-abc", cfg.RegistryToken)
}

// TestOptionsForRegistry_UserPass: username/password become a static keychain
// resolving to basic-auth credentials.
func TestOptionsForRegistry_UserPass(t *testing.T) {
	reg := &providermodels.RegistryV2{Username: strptr("alice"), Password: strptr("s3cret")}
	opts := OptionsForRegistry(reg)

	require.NotNil(t, opts.Keychain)
	auth, err := opts.Keychain.Resolve(testResource{})
	require.NoError(t, err)
	cfg, err := auth.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "alice", cfg.Username)
	assert.Equal(t, "s3cret", cfg.Password)
}

// TestOptionsForRegistry_TokenPreferredOverUserPass: when both are set the
// bearer token wins, matching registryAuthenticator's precedence.
func TestOptionsForRegistry_TokenPreferredOverUserPass(t *testing.T) {
	reg := &providermodels.RegistryV2{
		Token:    strptr("jwt-xyz"),
		Username: strptr("alice"),
		Password: strptr("s3cret"),
	}
	auth := registryAuthenticator(reg)
	require.NotNil(t, auth)
	cfg, err := auth.Authorization()
	require.NoError(t, err)
	assert.Equal(t, "jwt-xyz", cfg.RegistryToken)
	assert.Empty(t, cfg.Username)
}

// TestOptionsForRegistry_EmptyCredsNoKeychain: empty-string credentials (how
// NewRegistryV2FromEnv populates unset vars) are treated as absent, so no
// keychain is attached and the ambient docker config is used.
func TestOptionsForRegistry_EmptyCredsNoKeychain(t *testing.T) {
	reg := &providermodels.RegistryV2{
		Token:    strptr(""),
		Username: strptr(""),
		Password: strptr(""),
	}
	opts := OptionsForRegistry(reg)
	assert.Nil(t, opts.Keychain)
	assert.Nil(t, registryAuthenticator(reg))
}

// TestOptionsForRegistry_Insecure: Insecure propagates to Options.Insecure and,
// on its own, does not add a custom transport.
func TestOptionsForRegistry_Insecure(t *testing.T) {
	reg := &providermodels.RegistryV2{Insecure: true}
	opts := OptionsForRegistry(reg)
	assert.True(t, opts.Insecure)
	assert.Empty(t, opts.RemoteOptions)
}

// TestOptionsForRegistry_SkipTLS: SkipTLS attaches a custom remote transport
// (with certificate verification disabled).
func TestOptionsForRegistry_SkipTLS(t *testing.T) {
	reg := &providermodels.RegistryV2{SkipTLS: true}
	opts := OptionsForRegistry(reg)
	assert.NotEmpty(t, opts.RemoteOptions, "SkipTLS must inject a custom transport")
}

// TestStaticKeychain_Resolve: the static keychain returns its fixed
// authenticator for any resource.
func TestStaticKeychain_Resolve(t *testing.T) {
	want := &authn.Bearer{Token: "t"}
	k := staticKeychain{auth: want}
	got, err := k.Resolve(testResource{})
	require.NoError(t, err)
	assert.Same(t, want, got)
}
