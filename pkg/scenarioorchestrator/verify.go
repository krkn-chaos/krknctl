package scenarioorchestrator

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/verify"
)

// VerifyAndPinImage verifies the cosign signature of image using the shared
// ecosystem policy (pkg/verify) and returns the pinned digest reference that
// MUST be run in place of the original tag. It fails closed: any verification
// error (unsigned, invalid signature, registry unreachable, invalid reference)
// is returned unchanged so the caller aborts the run instead of executing an
// unverified image.
//
// Registry credentials and TLS settings from the (optional) private-registry
// configuration are mirrored into the verification request so a signed image in
// a private/insecure registry can be verified with the same access the pull path
// uses.
func VerifyAndPinImage(ctx context.Context, image string, registry *providermodels.RegistryV2) (string, error) {
	verified, err := verify.VerifyImage(ctx, image, verifyOptionsForRegistry(registry))
	if err != nil {
		return "", err
	}
	return verified.Digest, nil
}

// verifyOptionsForRegistry translates a RegistryV2 into verify.Options,
// mirroring the authentication and TLS behaviour of the pull path:
//   - bearer token, or username/password, as a static keychain;
//   - Insecure -> plain HTTP;
//   - SkipTLS  -> HTTPS with certificate verification disabled (custom transport).
//
// A nil registry (the default public registry) yields the zero Options, which
// use the ambient docker keychain over HTTPS.
func verifyOptionsForRegistry(registry *providermodels.RegistryV2) verify.Options {
	opts := verify.Options{}
	if registry == nil {
		return opts
	}

	if auth := registryAuthenticator(registry); auth != nil {
		opts.Keychain = staticKeychain{auth: auth}
	}

	opts.Insecure = registry.Insecure

	if registry.SkipTLS {
		transport := remote.DefaultTransport.(*http.Transport).Clone()
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{} // #nosec G402 -- user opted into skipping TLS verification for this registry
		}
		transport.TLSClientConfig.InsecureSkipVerify = true // #nosec G402 -- explicit user opt-in via registry SkipTLS
		opts.RemoteOptions = append(opts.RemoteOptions, remote.WithTransport(transport))
	}

	return opts
}

// registryAuthenticator builds a go-containerregistry authenticator from the
// registry credentials, preferring a bearer token when present. It returns nil
// when no usable credentials are configured (empty strings are treated as
// absent, matching how NewRegistryV2FromEnv populates the pointers).
func registryAuthenticator(registry *providermodels.RegistryV2) authn.Authenticator {
	if registry.Token != nil && *registry.Token != "" {
		return &authn.Bearer{Token: *registry.Token}
	}
	if registry.Username != nil && *registry.Username != "" {
		password := ""
		if registry.Password != nil {
			password = *registry.Password
		}
		return authn.FromConfig(authn.AuthConfig{
			Username: *registry.Username,
			Password: password,
		})
	}
	return nil
}

// staticKeychain resolves every registry request to a single fixed
// authenticator, mirroring the explicit credentials the pull path injects
// (which do not rely on the ambient docker config).
type staticKeychain struct {
	auth authn.Authenticator
}

func (k staticKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return k.auth, nil
}