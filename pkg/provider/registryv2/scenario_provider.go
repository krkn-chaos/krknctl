// Package registryv2 Package provides the implementation of the generic V2 Docker registry
package registryv2

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
)

// authChallenge represents a parsed WWW-Authenticate Bearer challenge from a Docker registry
type authChallenge struct {
	Realm   string // OAuth2 token endpoint URL
	Service string // Registry service identifier
	Scope   string // Repository and permission scope
}

// tokenResponse represents an OAuth2 token response following Docker Registry v2 spec
type tokenResponse struct {
	Token     string    `json:"token"`      // JWT bearer token
	ExpiresIn int       `json:"expires_in"` // Token lifetime in seconds
	IssuedAt  time.Time `json:"issued_at"`  // Token issue timestamp
}

type ScenarioProvider struct {
	provider.BaseScenarioProvider
}

func (s *ScenarioProvider) GetRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	return s.getRegistryImages(registry)
}

func (s *ScenarioProvider) getRegistryImages(registry *models.RegistryV2) (*[]models.ScenarioTag, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	registryURI, err := registry.GetV2ScenarioRepositoryAPIURI()
	if err != nil {
		return nil, err
	}

	body, err := s.queryRegistry(registryURI, registry.Username, registry.Password, registry.Token, "GET", registry.SkipTLS)
	if err != nil {
		return nil, err
	}
	v2Tags := TagsV2{}
	var tags []models.ScenarioTag
	err = json.Unmarshal(*body, &v2Tags)
	if err != nil {
		return nil, err
	}
	for _, tag := range v2Tags.Tags {
		t := models.ScenarioTag{}
		t.Name = tag
		tags = append(tags, t)
	}

	return &tags, nil
}

// parseWWWAuthenticate parses a Docker Registry v2 WWW-Authenticate Bearer challenge
// header and extracts the realm, service, and scope parameters required for OAuth2
// token acquisition.
//
// Example input:
//
//	Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/hello-world:pull"
//
// Returns an error if the header is malformed or missing the realm parameter.
func parseWWWAuthenticate(header string) (*authChallenge, error) {
	// Verify header starts with "Bearer "
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, fmt.Errorf("unsupported auth scheme (expected Bearer)")
	}

	// Extract key-value pairs using regex
	// Pattern matches: key="value"
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(header, -1)

	// Build challenge struct
	challenge := &authChallenge{}
	for _, match := range matches {
		if len(match) >= 3 {
			switch match[1] {
			case "realm":
				challenge.Realm = match[2]
			case "service":
				challenge.Service = match[2]
			case "scope":
				challenge.Scope = match[2]
			}
		}
	}

	// Validate required fields
	if challenge.Realm == "" {
		return nil, fmt.Errorf("missing realm in WWW-Authenticate")
	}

	return challenge, nil
}

// buildTokenCacheKey generates a cache key for an OAuth2 token based on registry URL and scope
func buildTokenCacheKey(registryURL, scope string) string {
	return fmt.Sprintf("oauth2_token:%s:%s", registryURL, scope)
}

// getCachedToken retrieves a token from cache if it exists and is still valid
func getCachedToken(cache interface{ GetString(string) *string }, cacheKey string) *tokenResponse {
	tokenJSON := cache.GetString(cacheKey)
	if tokenJSON == nil {
		return nil
	}

	var token tokenResponse
	if err := json.Unmarshal([]byte(*tokenJSON), &token); err != nil {
		// Invalid cache entry, ignore
		return nil
	}

	// Check expiry (with 60s safety margin)
	expiryTime := token.IssuedAt.Add(time.Duration(token.ExpiresIn-60) * time.Second)
	if time.Now().After(expiryTime) {
		// Token expired
		return nil
	}

	return &token
}

// cacheToken stores an OAuth2 token in the cache
func cacheToken(cache interface{ SetString(string, string) }, cacheKey string, token *tokenResponse) {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		// Don't fail the request if caching fails
		return
	}
	cache.SetString(cacheKey, string(tokenJSON))
}

// acquireOAuth2Token requests a bearer token from the OAuth2 endpoint specified in the challenge
func acquireOAuth2Token(challenge *authChallenge, username, password *string, skipTLS bool) (*tokenResponse, error) {
	// Build token request URL
	tokenURL, err := url.Parse(challenge.Realm)
	if err != nil {
		return nil, fmt.Errorf("invalid auth realm URL: %w", err)
	}

	params := url.Values{}
	if challenge.Service != "" {
		params.Add("service", challenge.Service)
	}
	if challenge.Scope != "" {
		params.Add("scope", challenge.Scope)
	}
	tokenURL.RawQuery = params.Encode()

	// Create HTTP client with TLS config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS,
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second, // Add timeout
	}

	// Build request with Basic Auth
	req, err := http.NewRequest("GET", tokenURL.String(), nil)
	if err != nil {
		return nil, err
	}

	if username != nil {
		pwd := ""
		if password != nil {
			pwd = *password
		}
		req.SetBasicAuth(*username, pwd)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse token response
	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Set issued_at if not provided
	if token.IssuedAt.IsZero() {
		token.IssuedAt = time.Now()
	}

	// Set default expiry if not provided
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = 300 // Default 5 minutes
	}

	return &token, nil
}

func (s *ScenarioProvider) queryRegistry(uri string, username *string, password *string, token *string, method string, skipTLS bool) (*[]byte, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid registry URL %q: %w", uri, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q in %q", parsedURL.Scheme, uri)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second, // Add timeout for safety
	}

	// Retry logic for OAuth2 flow
	retryCount := 0
	maxRetries := 1
	currentToken := token

	// Treat empty string tokens as nil
	if currentToken != nil && *currentToken == "" {
		currentToken = nil
	}

	for retryCount <= maxRetries {
		req, err := http.NewRequest(method, parsedURL.String(), nil)
		if err != nil {
			return nil, err
		}

		// Set authorization header
		if currentToken != nil {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *currentToken))
		} else if username != nil {
			registryPassword := ""
			if password != nil {
				registryPassword = *password
			}
			req.SetBasicAuth(*username, registryPassword)
		}

		resp, err := client.Do(req) // #nosec G704 -- URL is validated via url.Parse and scheme-allowlisted to http/https; source is user-supplied registry config in a CLI context, not external input
		if err != nil {
			return nil, err
		}

		// Handle 401 Unauthorized with OAuth2 flow
		if resp.StatusCode == http.StatusUnauthorized && retryCount < maxRetries {
			_ = resp.Body.Close() // #nosec G104 -- ignoring close error on retry path, body is being discarded

			// Skip OAuth2 flow if user already provided a token
			if token != nil {
				return nil, fmt.Errorf("authentication failed with provided token")
			}

			// Parse WWW-Authenticate challenge
			wwwAuth := resp.Header.Get("WWW-Authenticate")
			if wwwAuth == "" {
				return nil, fmt.Errorf("registry returned 401 without WWW-Authenticate header")
			}

			challenge, err := parseWWWAuthenticate(wwwAuth)
			if err != nil {
				return nil, fmt.Errorf("failed to parse auth challenge: %w", err)
			}

			// Check if credentials are available
			if username == nil {
				return nil, fmt.Errorf("authentication required but no credentials provided")
			}

			// Try to get cached token first
			cacheKey := buildTokenCacheKey(parsedURL.Host, challenge.Scope)
			tokenResp := getCachedToken(s.Cache, cacheKey)

			if tokenResp == nil {
				// Acquire new token
				tokenResp, err = acquireOAuth2Token(challenge, username, password, skipTLS)
				if err != nil {
					return nil, fmt.Errorf("failed to acquire OAuth2 token: %w", err)
				}

				// Cache the token
				cacheToken(s.Cache, cacheKey, tokenResp)
			}

			// Use the acquired token for retry
			currentToken = &tokenResp.Token
			retryCount++
			continue
		}

		// Defer body close for non-401 responses
		var deferErr error = nil
		defer func() {
			deferErr = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("image not found: %s", uri)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("URI %s returned %d: %s", uri, resp.StatusCode, string(bodyBytes))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		return &bodyBytes, deferErr
	}

	return nil, fmt.Errorf("maximum retry attempts exceeded")
}

func (s *ScenarioProvider) GetGlobalEnvironment(registry *models.RegistryV2, scenario string) (*models.ScenarioDetail, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	scenarioTags, err := s.getRegistryImages(registry)
	if err != nil {
		return nil, err
	}
	var foundScenario *models.ScenarioTag = nil
	for _, tag := range *scenarioTags {
		if tag.Name == scenario {
			foundScenario = &tag
		}
	}
	if foundScenario == nil {
		return nil, fmt.Errorf("%s scenario not found in registry %s", scenario, registry.RegistryURL)
	}
	baseImageRegistryURI, err := registry.GetV2ScenarioDetailAPIURI(foundScenario.Name)
	if err != nil {
		return nil, err
	}

	scenarioDetail, err := s.getScenarioDetail(baseImageRegistryURI, foundScenario, true, registry)
	if err != nil {
		return nil, err
	}
	return scenarioDetail, nil
}

func (s *ScenarioProvider) GetScenarioDetail(scenario string, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil in V2 scenario provider")
	}

	scenarios, err := s.GetRegistryImages(registry)
	if err != nil {
		return nil, err
	}
	var foundScenario *models.ScenarioTag = nil
	for _, scenarioTag := range *scenarios {
		if scenarioTag.Name == scenario {
			foundScenario = &scenarioTag
		}
	}
	if foundScenario == nil {
		return nil, nil
	}

	registryURI, err := registry.GetV2ScenarioDetailAPIURI(scenario)
	if err != nil {
		return nil, err
	}

	scenarioDetail, err := s.getScenarioDetail(registryURI, foundScenario, false, registry)
	if err != nil {
		return nil, err
	}
	return scenarioDetail, nil
}

func (s *ScenarioProvider) ScaffoldScenarios(scenarios []string, includeGlobalEnv bool, registry *models.RegistryV2, random bool, seed *provider.ScaffoldSeed) (*string, error) {
	return provider.ScaffoldScenarios(scenarios, includeGlobalEnv, registry, s.Config, s, random, seed)
}

func (s *ScenarioProvider) getScenarioDetail(dataSource string, foundScenario *models.ScenarioTag, isGlobalEnvironment bool, registry *models.RegistryV2) (*models.ScenarioDetail, error) {
	body, err := s.queryRegistry(dataSource, registry.Username, registry.Password, registry.Token, "GET", registry.SkipTLS)
	if err != nil {
		return nil, err
	}
	manifestV2 := ManifestV2{}
	if err = json.Unmarshal(*body, &manifestV2); err != nil {
		return nil, err
	}
	for _, l := range manifestV2.RawLayers {
		layer := LayerV1Compat{}
		if err = json.Unmarshal([]byte(l["v1Compatibility"]), &layer); err != nil {
			continue
		}
		manifestV2.Layers = append(manifestV2.Layers, layer)
	}
	scenarioDetail := models.ScenarioDetail{
		ScenarioTag: *foundScenario,
	}
	var titleLabel = ""
	var descriptionLabel = ""
	var inputFieldsLabel = ""
	if isGlobalEnvironment {
		titleLabel = s.Config.LabelTitleGlobal
		descriptionLabel = s.Config.LabelDescriptionGlobal
		inputFieldsLabel = s.Config.LabelInputFieldsGlobal

	} else {
		titleLabel = s.Config.LabelTitle
		descriptionLabel = s.Config.LabelDescription
		inputFieldsLabel = s.Config.LabelInputFields
	}
	var layers []provider.ContainerLayer
	for _, l := range manifestV2.Layers {
		layers = append(layers, l)
	}
	foundTitle := provider.GetKrknctlLabel(titleLabel, layers)
	foundDescription := provider.GetKrknctlLabel(descriptionLabel, layers)
	foundInputFields := provider.GetKrknctlLabel(inputFieldsLabel, layers)

	if foundTitle == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(titleLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}
	if foundDescription == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(descriptionLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}
	if foundInputFields == nil {
		return nil, fmt.Errorf("%s LABEL not found in tag: %s digest: %s", strings.Replace(inputFieldsLabel, "=", "", 1), foundScenario.Name, *foundScenario.Digest)
	}

	parsedTitle, err := s.BaseScenarioProvider.ParseTitle(*foundTitle, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}
	parsedDescription, err := s.BaseScenarioProvider.ParseDescription(*foundDescription, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	parsedInputFields, err := s.BaseScenarioProvider.ParseInputFields(*foundInputFields, isGlobalEnvironment)
	if err != nil {
		return nil, err
	}

	scenarioDetail.Title = *parsedTitle
	scenarioDetail.Description = *parsedDescription
	scenarioDetail.Fields = parsedInputFields
	return &scenarioDetail, nil

}
