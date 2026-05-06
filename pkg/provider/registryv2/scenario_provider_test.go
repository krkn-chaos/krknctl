package registryv2

import (
	json_parser "encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	krknctlconfig "github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/cache"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func getConfig(t *testing.T) krknctlconfig.Config {
	conf, err := krknctlconfig.LoadConfig()
	assert.Nil(t, err)
	return conf
}

func TestScenarioProvider_GetRegistryImages_PublicRegistry(t *testing.T) {
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	r := models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		SkipTLS:            true,
	}
	tags, err := p.GetRegistryImages(&r)
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)
}

/*
To authenticate with quay via jwt token:
curl -X GET \
  --user '<username>:password' \
  "https://quay.io/v2/auth?service=quay.io&scope=repository:rh_ee_tsebasti/krkn-hub-private:pull,push" \
  -k | jq -r '.token'
*/

func TestScenarioProvider_GetRegistryImages_jwt(t *testing.T) {
	utils.SkipTestIfForkPR(t)
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		t.Skip("QUAY_TOKEN not set, skipping test")
	}
	pr := models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTLS:            true,
	}

	tags, err := p.GetRegistryImages(&pr)
	if err != nil && strings.Contains(err.Error(), "authentication failed with provided token") {
		t.Skip("QUAY_TOKEN is invalid or expired, skipping test")
	}
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)

	quayToken = "wrong"
	pr = models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTLS:            true,
	}

	_, err = p.GetRegistryImages(&pr)
	assert.NotNil(t, err)
}

/*
To run an example registry with basic auth:

mkdir auth

docker run \
  --entrypoint htpasswd \
  httpd:2 -Bbn testuser testpassword > auth/htpasswd

docker run -d \
  -p 5001:5000 \
  --restart=always \
  --name registry \
  -v "$(pwd)"/auth:/auth \
  -e "REGISTRY_AUTH=htpasswd" \
  -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
  -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
  registry:2
*/

func TestScenarioProvider_GetRegistryImages_Htpasswd(t *testing.T) {
	basicAuthUsername := "testuser"
	basicAuthPassword := "testpassword"

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	pr := models.RegistryV2{
		RegistryURL:        "localhost:5001",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		Username:           &basicAuthUsername,
		Password:           &basicAuthPassword,
		Insecure:           true,
	}

	tags, err := p.GetRegistryImages(&pr)
	if err != nil && strings.Contains(err.Error(), "connection refused") {
		t.Skip("Local registry not running on localhost:5001, skipping test. See test comments for setup instructions.")
	}
	assert.Nil(t, err)
	assert.NotNil(t, tags)
	assert.Greater(t, len(*tags), 0)
	assert.Nil(t, (*tags)[0].Size)
	assert.Nil(t, (*tags)[0].LastModified)
	assert.Nil(t, (*tags)[0].Digest)

	basicAuthUsername = "wrong"
	basicAuthPassword = "wrong"
	pr = models.RegistryV2{
		RegistryURL:        "localhost:5001",
		ScenarioRepository: "krkn-chaos/krkn-hub",
		Username:           &basicAuthUsername,
		Password:           &basicAuthPassword,
		Insecure:           false,
	}
	_, err = p.GetRegistryImages(&pr)
	assert.NotNil(t, err)

}

func TestScenarioProvider_GetScenarioDetail(t *testing.T) {
	utils.SkipTestIfForkPR(t)
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		t.Skip("QUAY_TOKEN not set, skipping test")
	}
	pr := models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTLS:            false,
	}

	res, err := p.GetScenarioDetail("dummy-scenario", &pr)
	if err != nil && strings.Contains(err.Error(), "authentication failed with provided token") {
		t.Skip("QUAY_TOKEN is invalid or expired, skipping test")
	}
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.Name, "dummy-scenario")
	assert.Equal(t, res.Title, "Dummy Scenario")
	assert.Equal(t, len(res.Fields), 2)
	assert.NotNil(t, res.ScenarioTag.Name)
	assert.Nil(t, res.ScenarioTag.Size)
	assert.Nil(t, res.ScenarioTag.LastModified)
	assert.Nil(t, res.ScenarioTag.Digest)

}

func TestScenarioProvider_GetGlobalEnvironment(t *testing.T) {
	utils.SkipTestIfForkPR(t)
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		t.Skip("QUAY_TOKEN not set, skipping test")
	}
	pr := models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTLS:            false,
	}
	res, err := p.GetGlobalEnvironment(&pr, "dummy-scenario")
	if err != nil && strings.Contains(err.Error(), "authentication failed with provided token") {
		t.Skip("QUAY_TOKEN is invalid or expired, skipping test")
	}
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.Title, "Krkn Base Image")
	assert.Equal(t, res.Description, "This is the krkn base image.")
	assert.True(t, len(res.Fields) > 0)

	pr = models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		// does not contain any latest tag, error expected
		Token:   &quayToken,
		SkipTLS: true,
	}
	res, err = p.GetGlobalEnvironment(&pr, "")
	assert.NotNil(t, err)
	assert.Nil(t, res)
}

func TestScenarioProvider_ScaffoldScenarios(t *testing.T) {
	utils.SkipTestIfForkPR(t)
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}
	quayToken := os.Getenv("QUAY_TOKEN")
	if quayToken == "" {
		t.Skip("QUAY_TOKEN not set, skipping test")
	}
	pr := models.RegistryV2{
		RegistryURL:        "quay.io",
		ScenarioRepository: "rh_ee_tsebasti/krkn-hub-private",
		Token:              &quayToken,
		SkipTLS:            false,
	}

	scenarios, err := p.GetRegistryImages(&pr)
	if err != nil && strings.Contains(err.Error(), "authentication failed with provided token") {
		t.Skip("QUAY_TOKEN is invalid or expired, skipping test")
	}
	assert.Nil(t, err)
	assert.NotNil(t, scenarios)
	scenarioNames := []string{"node-cpu-hog", "node-memory-hog", "dummy-scenario"}

	json, err := p.ScaffoldScenarios(scenarioNames, false, &pr, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = p.ScaffoldScenarios(scenarioNames, false, &pr, true, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)
	var parsedScenarios map[string]map[string]interface{}
	err = json_parser.Unmarshal([]byte(*json), &parsedScenarios)
	assert.Nil(t, err)
	for el := range parsedScenarios {
		assert.NotEqual(t, el, "comment")
		_, ok := parsedScenarios[el]["depends_on"]
		assert.False(t, ok)
	}

	json, err = p.ScaffoldScenarios(scenarioNames, false, &pr, false, nil)
	assert.Nil(t, err)
	assert.NotNil(t, json)

	json, err = p.ScaffoldScenarios([]string{"node-cpu-hog", "does-not-exist"}, false, &pr, false, nil)
	assert.Nil(t, json)
	assert.NotNil(t, err)
}

// OAuth2 Unit Tests

func TestParseWWWAuthenticate_Valid(t *testing.T) {
	header := `Bearer realm="https://auth.example.com/token",service="registry.example.com",scope="repository:library/image:pull"`
	challenge, err := parseWWWAuthenticate(header)

	assert.Nil(t, err)
	assert.NotNil(t, challenge)
	assert.Equal(t, "https://auth.example.com/token", challenge.Realm)
	assert.Equal(t, "registry.example.com", challenge.Service)
	assert.Equal(t, "repository:library/image:pull", challenge.Scope)
}

func TestParseWWWAuthenticate_MissingRealm(t *testing.T) {
	header := `Bearer service="registry.example.com",scope="repository:library/image:pull"`
	challenge, err := parseWWWAuthenticate(header)

	assert.NotNil(t, err)
	assert.Nil(t, challenge)
	assert.Contains(t, err.Error(), "missing realm")
}

func TestParseWWWAuthenticate_OptionalScope(t *testing.T) {
	header := `Bearer realm="https://auth.example.com/token",service="registry.example.com"`
	challenge, err := parseWWWAuthenticate(header)

	assert.Nil(t, err)
	assert.NotNil(t, challenge)
	assert.Equal(t, "https://auth.example.com/token", challenge.Realm)
	assert.Equal(t, "registry.example.com", challenge.Service)
	assert.Equal(t, "", challenge.Scope) // Scope is optional
}

func TestParseWWWAuthenticate_InvalidScheme(t *testing.T) {
	header := `Basic realm="https://auth.example.com/token"`
	challenge, err := parseWWWAuthenticate(header)

	assert.NotNil(t, err)
	assert.Nil(t, challenge)
	assert.Contains(t, err.Error(), "unsupported auth scheme")
}

func TestBuildTokenCacheKey(t *testing.T) {
	key1 := buildTokenCacheKey("quay.io", "repository:org/repo:pull")
	key2 := buildTokenCacheKey("quay.io", "repository:org/repo:pull")
	key3 := buildTokenCacheKey("quay.io", "repository:org/other:pull")

	// Same inputs should produce same key
	assert.Equal(t, key1, key2)

	// Different scope should produce different key
	assert.NotEqual(t, key1, key3)

	// Key should follow expected format
	assert.Contains(t, key1, "oauth2_token:")
	assert.Contains(t, key1, "quay.io")
}

func TestGetCachedToken_Valid(t *testing.T) {
	cache := cache.NewCache()
	cacheKey := "test-key"

	// Create a token with future expiry
	token := &tokenResponse{
		Token:     "test-token-123",
		ExpiresIn: 300, // 5 minutes
		IssuedAt:  time.Now(),
	}

	// Cache the token
	cacheToken(cache, cacheKey, token)

	// Retrieve cached token
	cachedToken := getCachedToken(cache, cacheKey)

	assert.NotNil(t, cachedToken)
	assert.Equal(t, "test-token-123", cachedToken.Token)
	assert.Equal(t, 300, cachedToken.ExpiresIn)
}

func TestGetCachedToken_Expired(t *testing.T) {
	cache := cache.NewCache()
	cacheKey := "test-key-expired"

	// Create a token that expired 10 seconds ago
	token := &tokenResponse{
		Token:     "expired-token",
		ExpiresIn: 60, // 60 seconds expiry
		IssuedAt:  time.Now().Add(-70 * time.Second), // Issued 70 seconds ago
	}

	// Cache the expired token
	cacheToken(cache, cacheKey, token)

	// Try to retrieve - should return nil because it's expired
	cachedToken := getCachedToken(cache, cacheKey)

	assert.Nil(t, cachedToken)
}

func TestGetCachedToken_InvalidJSON(t *testing.T) {
	cache := cache.NewCache()
	cacheKey := "test-key-invalid"

	// Store invalid JSON in cache
	cache.SetString(cacheKey, "not-valid-json{")

	// Should return nil for malformed cache entry
	cachedToken := getCachedToken(cache, cacheKey)

	assert.Nil(t, cachedToken)
}

func TestGetCachedToken_NotFound(t *testing.T) {
	cache := cache.NewCache()

	// Try to get non-existent token
	cachedToken := getCachedToken(cache, "non-existent-key")

	assert.Nil(t, cachedToken)
}

// OAuth2 Integration Tests

// startMockOAuth2Server creates a mock OAuth2 token endpoint for testing
func startMockOAuth2Server(t *testing.T, validUsername, validPassword string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()

		if !ok || username != validUsername || password != validPassword {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_credentials"}`))
			return
		}

		token := tokenResponse{
			Token:     "test-token-" + time.Now().Format("20060102150405"),
			ExpiresIn: 300,
			IssuedAt:  time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		json_parser.NewEncoder(w).Encode(token)
	}))
}

func TestOAuth2Flow_Success(t *testing.T) {
	// Start mock OAuth2 server
	authServer := startMockOAuth2Server(t, "testuser", "testpass")
	defer authServer.Close()

	// Create mock registry server that returns 401 with WWW-Authenticate
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" || !strings.Contains(authHeader, "Bearer") {
			// First request without token - return 401 with challenge
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+authServer.URL+`",service="test-registry",scope="repository:test/repo:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		// Second request with token - return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":["latest","v1.0"]}`))
	}))
	defer registryServer.Close()

	// Create scenario provider with cache
	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	username := "testuser"
	password := "testpass"

	// Make request - should automatically acquire token and retry
	body, err := p.queryRegistry(registryServer.URL+"/v2/test/repo/tags/list", &username, &password, nil, "GET", false)

	assert.Nil(t, err)
	assert.NotNil(t, body)
	assert.Contains(t, string(*body), "latest")
}

func TestOAuth2Flow_CachedToken(t *testing.T) {
	tokenRequestCount := 0

	// Mock OAuth2 server that counts requests
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequestCount++
		token := tokenResponse{
			Token:     "cached-token",
			ExpiresIn: 300,
			IssuedAt:  time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json_parser.NewEncoder(w).Encode(token)
	}))
	defer authServer.Close()

	// Mock registry server
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// Only accept Bearer tokens, reject basic auth
		if authHeader == "" || !strings.Contains(authHeader, "Bearer") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+authServer.URL+`",service="test-registry",scope="repository:test/repo:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":["latest"]}`))
	}))
	defer registryServer.Close()

	// Create scenario provider with shared cache
	config := getConfig(t)
	sharedCache := cache.NewCache()
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  sharedCache,
		},
	}

	username := "testuser"
	password := "testpass"

	// First request - acquires token
	_, err := p.queryRegistry(registryServer.URL+"/v2/test/repo/tags/list", &username, &password, nil, "GET", false)
	assert.Nil(t, err)
	assert.Equal(t, 1, tokenRequestCount)

	// Second request - should use cached token
	_, err = p.queryRegistry(registryServer.URL+"/v2/test/repo/tags/list", &username, &password, nil, "GET", false)
	assert.Nil(t, err)
	assert.Equal(t, 1, tokenRequestCount) // Should still be 1 (no new token request)
}

func TestOAuth2Flow_NoCredentials(t *testing.T) {
	// Mock registry returning 401
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="https://auth.example.com/token",service="test-registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer registryServer.Close()

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	// No credentials provided
	_, err := p.queryRegistry(registryServer.URL+"/v2/test/tags/list", nil, nil, nil, "GET", false)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "authentication required but no credentials provided")
}

func TestOAuth2Flow_InvalidCredentials(t *testing.T) {
	// Mock OAuth2 server that rejects credentials
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_credentials"}`))
	}))
	defer authServer.Close()

	// Mock registry returning 401
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+authServer.URL+`",service="test-registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer registryServer.Close()

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	username := "wronguser"
	password := "wrongpass"

	_, err := p.queryRegistry(registryServer.URL+"/v2/test/tags/list", &username, &password, nil, "GET", false)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to acquire OAuth2 token")
}

func TestOAuth2Flow_PreProvidedToken(t *testing.T) {
	// Mock registry that rejects the token
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer registryServer.Close()

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	username := "testuser"
	password := "testpass"
	invalidToken := "invalid-token-123"

	// Should fail immediately without trying OAuth2
	_, err := p.queryRegistry(registryServer.URL+"/v2/test/tags/list", &username, &password, &invalidToken, "GET", false)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "authentication failed with provided token")
}

func TestOAuth2Flow_BackwardCompatibility_BasicAuth(t *testing.T) {
	// Mock registry that accepts basic auth (no 401)
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()

		if !ok || username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Accept basic auth directly
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"tags":["latest"]}`))
	}))
	defer registryServer.Close()

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	username := "testuser"
	password := "testpass"

	// Should succeed with basic auth on first try (no OAuth2 flow)
	body, err := p.queryRegistry(registryServer.URL+"/v2/test/tags/list", &username, &password, nil, "GET", false)

	assert.Nil(t, err)
	assert.NotNil(t, body)
	assert.Contains(t, string(*body), "latest")
}

func TestOAuth2Flow_No_WWWAuthenticate_Header(t *testing.T) {
	// Mock registry returning 401 WITHOUT WWW-Authenticate header
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer registryServer.Close()

	config := getConfig(t)
	p := ScenarioProvider{
		provider.BaseScenarioProvider{
			Config: config,
			Cache:  cache.NewCache(),
		},
	}

	username := "testuser"
	password := "testpass"

	_, err := p.queryRegistry(registryServer.URL+"/v2/test/tags/list", &username, &password, nil, "GET", false)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "registry returned 401 without WWW-Authenticate header")
}
