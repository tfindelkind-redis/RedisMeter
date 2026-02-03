// Package auth provides authentication and authorization for RedisMeter.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// OIDCConfig holds configuration for OIDC authentication.
type OIDCConfig struct {
	// Provider name (e.g., "google", "github", "azure", "okta")
	Provider string `json:"provider"`

	// OAuth2/OIDC endpoints
	Issuer            string `json:"issuer"`
	AuthorizationURL  string `json:"authorization_url"`
	TokenURL          string `json:"token_url"`
	UserInfoURL       string `json:"userinfo_url"`
	JWKsURL           string `json:"jwks_url"`

	// Client credentials
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`

	// Redirect URI for OAuth flow
	RedirectURI string `json:"redirect_uri"`

	// Scopes to request
	Scopes []string `json:"scopes"`

	// Token settings
	TokenTTL   time.Duration `json:"token_ttl"`
	RefreshTTL time.Duration `json:"refresh_ttl"`

	// User mapping
	EmailClaim    string `json:"email_claim"`
	NameClaim     string `json:"name_claim"`
	GroupsClaim   string `json:"groups_claim"`

	// Auto-provisioning
	AutoProvision   bool              `json:"auto_provision"`
	DefaultOrgID    string            `json:"default_org_id"`
	DefaultRole     Role              `json:"default_role"`
	GroupRoleMapping map[string]Role  `json:"group_role_mapping"`
}

// DefaultOIDCConfigs returns pre-configured settings for common providers.
func DefaultOIDCConfigs() map[string]OIDCConfig {
	return map[string]OIDCConfig{
		"google": {
			Provider:         "google",
			Issuer:           "https://accounts.google.com",
			AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:         "https://oauth2.googleapis.com/token",
			UserInfoURL:      "https://openidconnect.googleapis.com/v1/userinfo",
			JWKsURL:          "https://www.googleapis.com/oauth2/v3/certs",
			Scopes:           []string{"openid", "email", "profile"},
			EmailClaim:       "email",
			NameClaim:        "name",
		},
		"github": {
			Provider:         "github",
			AuthorizationURL: "https://github.com/login/oauth/authorize",
			TokenURL:         "https://github.com/login/oauth/access_token",
			UserInfoURL:      "https://api.github.com/user",
			Scopes:           []string{"read:user", "user:email"},
			EmailClaim:       "email",
			NameClaim:        "name",
		},
		"azure": {
			Provider:         "azure",
			// Tenant-specific URLs configured at runtime
			Scopes:       []string{"openid", "email", "profile"},
			EmailClaim:   "email",
			NameClaim:    "name",
			GroupsClaim:  "groups",
		},
		"okta": {
			Provider:    "okta",
			// Domain-specific URLs configured at runtime
			Scopes:      []string{"openid", "email", "profile", "groups"},
			EmailClaim:  "email",
			NameClaim:   "name",
			GroupsClaim: "groups",
		},
	}
}

// OIDCProvider implements OAuth2/OIDC authentication.
type OIDCProvider struct {
	config     OIDCConfig
	httpClient *http.Client
	mu         sync.RWMutex

	// State management for OAuth flow
	states map[string]*oauthState

	// Token storage
	tokens map[string]*tokenRecord

	// User cache
	users map[string]*User
}

type oauthState struct {
	Nonce     string
	CreatedAt time.Time
	RedirectTo string
}

// NewOIDCProvider creates a new OIDC authentication provider.
func NewOIDCProvider(config OIDCConfig) (*OIDCProvider, error) {
	// Apply defaults
	if config.TokenTTL == 0 {
		config.TokenTTL = 1 * time.Hour
	}
	if config.RefreshTTL == 0 {
		config.RefreshTTL = 7 * 24 * time.Hour
	}
	if config.EmailClaim == "" {
		config.EmailClaim = "email"
	}
	if config.NameClaim == "" {
		config.NameClaim = "name"
	}
	if config.DefaultRole == "" {
		config.DefaultRole = RoleMember
	}

	return &OIDCProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		states: make(map[string]*oauthState),
		tokens: make(map[string]*tokenRecord),
		users:  make(map[string]*User),
	}, nil
}

// Metadata returns plugin metadata.
func (p *OIDCProvider) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        fmt.Sprintf("oidc-%s", p.config.Provider),
		Version:     "1.0.0",
		Type:        plugin.Type("auth"),
		Description: fmt.Sprintf("OAuth2/OIDC authentication via %s", p.config.Provider),
	}
}

// Initialize sets up the plugin.
func (p *OIDCProvider) Initialize(ctx context.Context, config map[string]interface{}) error {
	// Could fetch OIDC discovery document here
	return nil
}

// HealthCheck returns the plugin health status.
func (p *OIDCProvider) HealthCheck(ctx context.Context) plugin.HealthStatus {
	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (p *OIDCProvider) Shutdown(ctx context.Context) error {
	return nil
}

// GetAuthorizationURL generates the OAuth authorization URL.
func (p *OIDCProvider) GetAuthorizationURL(redirectTo string) (string, string, error) {
	state := generateState()
	nonce := generateState()

	p.mu.Lock()
	p.states[state] = &oauthState{
		Nonce:      nonce,
		CreatedAt:  time.Now(),
		RedirectTo: redirectTo,
	}
	p.mu.Unlock()

	params := url.Values{
		"client_id":     {p.config.ClientID},
		"redirect_uri":  {p.config.RedirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(p.config.Scopes, " ")},
		"state":         {state},
		"nonce":         {nonce},
	}

	authURL := fmt.Sprintf("%s?%s", p.config.AuthorizationURL, params.Encode())
	return authURL, state, nil
}

// HandleCallback processes the OAuth callback and returns a user.
func (p *OIDCProvider) HandleCallback(ctx context.Context, code, state string) (*User, *Token, error) {
	// Validate state
	p.mu.Lock()
	stateRecord, exists := p.states[state]
	if exists {
		delete(p.states, state)
	}
	p.mu.Unlock()

	if !exists {
		return nil, nil, fmt.Errorf("invalid state parameter")
	}

	// Check state expiration (5 minutes)
	if time.Since(stateRecord.CreatedAt) > 5*time.Minute {
		return nil, nil, fmt.Errorf("state has expired")
	}

	// Exchange code for tokens
	tokenResp, err := p.exchangeCode(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user info
	user, err := p.getUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Store user
	p.mu.Lock()
	p.users[user.ID] = user
	p.mu.Unlock()

	// Create internal token
	internalToken := generateToken()
	p.mu.Lock()
	p.tokens[internalToken] = &tokenRecord{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(p.config.TokenTTL),
		Scopes:    []string{"*"},
	}
	p.mu.Unlock()

	token := &Token{
		AccessToken:  internalToken,
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(p.config.TokenTTL),
		RefreshToken: tokenResp.RefreshToken,
		Scopes:       []string{"*"},
	}

	return user, token, nil
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

func (p *OIDCProvider) exchangeCode(ctx context.Context, code string) (*oauthTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.config.RedirectURI},
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func (p *OIDCProvider) getUserInfo(ctx context.Context, accessToken string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: %s", string(body))
	}

	var claims map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}

	// Extract user info from claims
	email := getStringClaim(claims, p.config.EmailClaim)
	name := getStringClaim(claims, p.config.NameClaim)
	sub := getStringClaim(claims, "sub")

	if email == "" && sub == "" {
		return nil, fmt.Errorf("no email or sub claim in response")
	}

	userID := sub
	if userID == "" {
		userID = email
	}

	now := time.Now()
	user := &User{
		ID:        userID,
		Email:     email,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		Active:    true,
	}

	// Apply group-role mapping if configured
	if p.config.GroupsClaim != "" && p.config.GroupRoleMapping != nil {
		groups := getStringArrayClaim(claims, p.config.GroupsClaim)
		for _, group := range groups {
			if role, ok := p.config.GroupRoleMapping[group]; ok {
				user.OrgRole = role
				break
			}
		}
	}

	if user.OrgRole == "" {
		user.OrgRole = p.config.DefaultRole
	}

	if p.config.DefaultOrgID != "" {
		user.OrgID = p.config.DefaultOrgID
	}

	return user, nil
}

// Authenticate is not used for OIDC (use HandleCallback instead).
func (p *OIDCProvider) Authenticate(ctx context.Context, creds Credentials) (*User, error) {
	return nil, fmt.Errorf("OIDC provider does not support direct authentication; use OAuth flow")
}

// ValidateToken validates an access token and returns the associated user.
func (p *OIDCProvider) ValidateToken(ctx context.Context, token string) (*User, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.tokens[token]
	if !exists {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(record.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	user, exists := p.users[record.UserID]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// RefreshToken generates a new access token using a refresh token.
func (p *OIDCProvider) RefreshToken(ctx context.Context, refreshToken string) (*Token, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Create internal token
	internalToken := generateToken()
	
	// We need to get user ID from somewhere - this is simplified
	// In production, decode the ID token or maintain mapping

	return &Token{
		AccessToken:  internalToken,
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(p.config.TokenTTL),
		RefreshToken: tokenResp.RefreshToken,
		Scopes:       []string{"*"},
	}, nil
}

// Logout invalidates a token.
func (p *OIDCProvider) Logout(ctx context.Context, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.tokens, token)
	return nil
}

// CreateAPIKey creates a new API key for a user.
func (p *OIDCProvider) CreateAPIKey(ctx context.Context, userID string, name string, scopes []string, expiresIn time.Duration) (*APIKey, string, error) {
	// OIDC provider delegates to local storage for API keys
	return nil, "", fmt.Errorf("API key management not implemented for OIDC provider")
}

// ValidateAPIKey validates an API key and returns the associated user.
func (p *OIDCProvider) ValidateAPIKey(ctx context.Context, key string) (*User, *APIKey, error) {
	return nil, nil, fmt.Errorf("API key validation not implemented for OIDC provider")
}

// RevokeAPIKey revokes an API key.
func (p *OIDCProvider) RevokeAPIKey(ctx context.Context, keyID string) error {
	return fmt.Errorf("API key revocation not implemented for OIDC provider")
}

// Helper functions

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func getStringClaim(claims map[string]interface{}, key string) string {
	if val, ok := claims[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getStringArrayClaim(claims map[string]interface{}, key string) []string {
	if val, ok := claims[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			var result []string
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// Ensure OIDCProvider implements AuthProvider.
var _ AuthProvider = (*OIDCProvider)(nil)
