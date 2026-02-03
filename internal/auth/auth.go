// Package auth provides authentication and authorization for RedisMeter.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenInvalid       = errors.New("invalid token")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrAPIKeyExpired      = errors.New("API key has expired")
	ErrAPIKeyDisabled     = errors.New("API key is disabled")
)

// User represents an authenticated user.
type User struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Name      string            `json:"name,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	LastLogin time.Time         `json:"last_login,omitempty"`
	Active    bool              `json:"active"`
	Metadata  map[string]string `json:"metadata,omitempty"`

	// Organization membership
	OrgID   string `json:"org_id,omitempty"`
	OrgRole Role   `json:"org_role,omitempty"`

	// Team memberships
	Teams []TeamMembership `json:"teams,omitempty"`
}

// TeamMembership represents a user's membership in a team.
type TeamMembership struct {
	TeamID   string    `json:"team_id"`
	TeamName string    `json:"team_name"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Role represents a user's role in an organization or team.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Permission represents an action that can be performed.
type Permission string

const (
	PermissionRunCreate   Permission = "run:create"
	PermissionRunRead     Permission = "run:read"
	PermissionRunUpdate   Permission = "run:update"
	PermissionRunDelete   Permission = "run:delete"
	PermissionBaselineManage Permission = "baseline:manage"
	PermissionTeamManage  Permission = "team:manage"
	PermissionOrgManage   Permission = "org:manage"
	PermissionAPIKeyManage Permission = "apikey:manage"
	PermissionAuditRead   Permission = "audit:read"
)

// Token represents an authentication token.
type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

// APIKey represents a programmatic access key.
type APIKey struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Prefix     string    `json:"prefix"` // First few chars for identification
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	Scopes     []string  `json:"scopes"`
	Active     bool      `json:"active"`
}

// Credentials represents login credentials.
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthProvider defines the interface for authentication providers.
type AuthProvider interface {
	plugin.Plugin

	// Authenticate validates credentials and returns a user.
	Authenticate(ctx context.Context, creds Credentials) (*User, error)

	// ValidateToken validates an access token and returns the associated user.
	ValidateToken(ctx context.Context, token string) (*User, error)

	// RefreshToken generates a new access token using a refresh token.
	RefreshToken(ctx context.Context, refreshToken string) (*Token, error)

	// Logout invalidates a token.
	Logout(ctx context.Context, token string) error

	// CreateAPIKey creates a new API key for a user.
	CreateAPIKey(ctx context.Context, userID string, name string, scopes []string, expiresIn time.Duration) (*APIKey, string, error)

	// ValidateAPIKey validates an API key and returns the associated user.
	ValidateAPIKey(ctx context.Context, key string) (*User, *APIKey, error)

	// RevokeAPIKey revokes an API key.
	RevokeAPIKey(ctx context.Context, keyID string) error
}

// Authorizer defines the interface for authorization.
type Authorizer interface {
	// HasPermission checks if a user has a specific permission.
	HasPermission(ctx context.Context, user *User, permission Permission, resourceID string) bool

	// GetPermissions returns all permissions for a user.
	GetPermissions(ctx context.Context, user *User) []Permission

	// CheckResourceAccess verifies if a user can access a specific resource.
	CheckResourceAccess(ctx context.Context, user *User, resourceType string, resourceID string, action string) error
}

// LocalAuthProvider implements local username/password authentication.
type LocalAuthProvider struct {
	mu           sync.RWMutex
	users        map[string]*userRecord
	tokens       map[string]*tokenRecord
	apiKeys      map[string]*apiKeyRecord
	tokenTTL     time.Duration
	refreshTTL   time.Duration
	apiKeyPrefix string
}

type userRecord struct {
	User         User
	PasswordHash string
}

type tokenRecord struct {
	UserID    string
	ExpiresAt time.Time
	Scopes    []string
}

type apiKeyRecord struct {
	APIKey  APIKey
	KeyHash string
}

// LocalAuthConfig holds configuration for local authentication.
type LocalAuthConfig struct {
	TokenTTL     time.Duration `json:"token_ttl"`
	RefreshTTL   time.Duration `json:"refresh_ttl"`
	APIKeyPrefix string        `json:"api_key_prefix"`
}

// DefaultLocalAuthConfig returns sensible defaults.
func DefaultLocalAuthConfig() LocalAuthConfig {
	return LocalAuthConfig{
		TokenTTL:     1 * time.Hour,
		RefreshTTL:   7 * 24 * time.Hour,
		APIKeyPrefix: "rm_",
	}
}

// NewLocalAuthProvider creates a new local authentication provider.
func NewLocalAuthProvider(config LocalAuthConfig) *LocalAuthProvider {
	if config.TokenTTL == 0 {
		config.TokenTTL = 1 * time.Hour
	}
	if config.RefreshTTL == 0 {
		config.RefreshTTL = 7 * 24 * time.Hour
	}
	if config.APIKeyPrefix == "" {
		config.APIKeyPrefix = "rm_"
	}

	return &LocalAuthProvider{
		users:        make(map[string]*userRecord),
		tokens:       make(map[string]*tokenRecord),
		apiKeys:      make(map[string]*apiKeyRecord),
		tokenTTL:     config.TokenTTL,
		refreshTTL:   config.RefreshTTL,
		apiKeyPrefix: config.APIKeyPrefix,
	}
}

// Metadata returns plugin metadata.
func (p *LocalAuthProvider) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "local",
		Version:     "1.0.0",
		Type:        plugin.Type("auth"),
		Description: "Local username/password authentication",
	}
}

// Initialize sets up the plugin.
func (p *LocalAuthProvider) Initialize(ctx context.Context, config map[string]interface{}) error {
	return nil
}

// HealthCheck returns the plugin health status.
func (p *LocalAuthProvider) HealthCheck(ctx context.Context) plugin.HealthStatus {
	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (p *LocalAuthProvider) Shutdown(ctx context.Context) error {
	return nil
}

// CreateUser creates a new user account.
func (p *LocalAuthProvider) CreateUser(ctx context.Context, email, password, name string) (*User, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if user exists
	for _, record := range p.users {
		if record.User.Email == email {
			return nil, fmt.Errorf("user already exists: %s", email)
		}
	}

	id := generateID()
	now := time.Now()

	user := User{
		ID:        id,
		Email:     email,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		Active:    true,
	}

	p.users[id] = &userRecord{
		User:         user,
		PasswordHash: hashPassword(password),
	}

	return &user, nil
}

// Authenticate validates credentials and returns a user.
func (p *LocalAuthProvider) Authenticate(ctx context.Context, creds Credentials) (*User, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, record := range p.users {
		if record.User.Email == creds.Email {
			if !record.User.Active {
				return nil, ErrUserDisabled
			}
			if hashPassword(creds.Password) != record.PasswordHash {
				return nil, ErrInvalidCredentials
			}
			return &record.User, nil
		}
	}

	return nil, ErrInvalidCredentials
}

// Login authenticates and returns tokens.
func (p *LocalAuthProvider) Login(ctx context.Context, creds Credentials) (*Token, error) {
	user, err := p.Authenticate(ctx, creds)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update last login
	if record, ok := p.users[user.ID]; ok {
		record.User.LastLogin = time.Now()
	}

	// Generate tokens
	accessToken := generateToken()
	refreshToken := generateToken()
	now := time.Now()

	p.tokens[accessToken] = &tokenRecord{
		UserID:    user.ID,
		ExpiresAt: now.Add(p.tokenTTL),
		Scopes:    []string{"*"},
	}

	p.tokens[refreshToken] = &tokenRecord{
		UserID:    user.ID,
		ExpiresAt: now.Add(p.refreshTTL),
		Scopes:    []string{"refresh"},
	}

	return &Token{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresAt:    now.Add(p.tokenTTL),
		RefreshToken: refreshToken,
		Scopes:       []string{"*"},
	}, nil
}

// ValidateToken validates an access token and returns the associated user.
func (p *LocalAuthProvider) ValidateToken(ctx context.Context, token string) (*User, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.tokens[token]
	if !exists {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(record.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	userRecord, exists := p.users[record.UserID]
	if !exists {
		return nil, ErrUserNotFound
	}

	if !userRecord.User.Active {
		return nil, ErrUserDisabled
	}

	return &userRecord.User, nil
}

// RefreshToken generates a new access token using a refresh token.
func (p *LocalAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	record, exists := p.tokens[refreshToken]
	if !exists {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(record.ExpiresAt) {
		delete(p.tokens, refreshToken)
		return nil, ErrTokenExpired
	}

	// Check if this is a refresh token
	isRefreshToken := false
	for _, scope := range record.Scopes {
		if scope == "refresh" {
			isRefreshToken = true
			break
		}
	}
	if !isRefreshToken {
		return nil, ErrTokenInvalid
	}

	// Generate new access token
	accessToken := generateToken()
	now := time.Now()

	p.tokens[accessToken] = &tokenRecord{
		UserID:    record.UserID,
		ExpiresAt: now.Add(p.tokenTTL),
		Scopes:    []string{"*"},
	}

	return &Token{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresAt:    now.Add(p.tokenTTL),
		RefreshToken: refreshToken,
		Scopes:       []string{"*"},
	}, nil
}

// Logout invalidates a token.
func (p *LocalAuthProvider) Logout(ctx context.Context, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.tokens, token)
	return nil
}

// CreateAPIKey creates a new API key for a user.
func (p *LocalAuthProvider) CreateAPIKey(ctx context.Context, userID string, name string, scopes []string, expiresIn time.Duration) (*APIKey, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.users[userID]; !exists {
		return nil, "", ErrUserNotFound
	}

	// Generate API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	rawKey := p.apiKeyPrefix + base64.URLEncoding.EncodeToString(keyBytes)
	keyHash := hashAPIKey(rawKey)
	keyID := generateID()

	now := time.Now()
	apiKey := APIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      name,
		Prefix:    rawKey[:len(p.apiKeyPrefix)+8],
		CreatedAt: now,
		Scopes:    scopes,
		Active:    true,
	}

	if expiresIn > 0 {
		apiKey.ExpiresAt = now.Add(expiresIn)
	}

	p.apiKeys[keyID] = &apiKeyRecord{
		APIKey:  apiKey,
		KeyHash: keyHash,
	}

	return &apiKey, rawKey, nil
}

// ValidateAPIKey validates an API key and returns the associated user.
func (p *LocalAuthProvider) ValidateAPIKey(ctx context.Context, key string) (*User, *APIKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keyHash := hashAPIKey(key)

	for _, record := range p.apiKeys {
		if record.KeyHash == keyHash {
			if !record.APIKey.Active {
				return nil, nil, ErrAPIKeyDisabled
			}
			if !record.APIKey.ExpiresAt.IsZero() && time.Now().After(record.APIKey.ExpiresAt) {
				return nil, nil, ErrAPIKeyExpired
			}

			userRecord, exists := p.users[record.APIKey.UserID]
			if !exists {
				return nil, nil, ErrUserNotFound
			}

			if !userRecord.User.Active {
				return nil, nil, ErrUserDisabled
			}

			// Update last used (would need write lock in production)
			record.APIKey.LastUsedAt = time.Now()

			return &userRecord.User, &record.APIKey, nil
		}
	}

	return nil, nil, ErrTokenInvalid
}

// RevokeAPIKey revokes an API key.
func (p *LocalAuthProvider) RevokeAPIKey(ctx context.Context, keyID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if record, exists := p.apiKeys[keyID]; exists {
		record.APIKey.Active = false
		return nil
	}

	return fmt.Errorf("API key not found: %s", keyID)
}

// ListAPIKeys returns all API keys for a user.
func (p *LocalAuthProvider) ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var keys []*APIKey
	for _, record := range p.apiKeys {
		if record.APIKey.UserID == userID {
			keys = append(keys, &record.APIKey)
		}
	}

	return keys, nil
}

// RBACAuthorizer implements role-based access control.
type RBACAuthorizer struct {
	rolePermissions map[Role][]Permission
}

// NewRBACAuthorizer creates a new RBAC authorizer with default permissions.
func NewRBACAuthorizer() *RBACAuthorizer {
	return &RBACAuthorizer{
		rolePermissions: map[Role][]Permission{
			RoleOwner: {
				PermissionRunCreate, PermissionRunRead, PermissionRunUpdate, PermissionRunDelete,
				PermissionBaselineManage, PermissionTeamManage, PermissionOrgManage,
				PermissionAPIKeyManage, PermissionAuditRead,
			},
			RoleAdmin: {
				PermissionRunCreate, PermissionRunRead, PermissionRunUpdate, PermissionRunDelete,
				PermissionBaselineManage, PermissionTeamManage, PermissionAPIKeyManage, PermissionAuditRead,
			},
			RoleMember: {
				PermissionRunCreate, PermissionRunRead, PermissionRunUpdate,
				PermissionBaselineManage, PermissionAPIKeyManage,
			},
			RoleViewer: {
				PermissionRunRead,
			},
		},
	}
}

// HasPermission checks if a user has a specific permission.
func (a *RBACAuthorizer) HasPermission(ctx context.Context, user *User, permission Permission, resourceID string) bool {
	if user == nil {
		return false
	}

	permissions := a.GetPermissions(ctx, user)
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

// GetPermissions returns all permissions for a user.
func (a *RBACAuthorizer) GetPermissions(ctx context.Context, user *User) []Permission {
	if user == nil {
		return nil
	}

	// Get permissions from org role
	permissions := a.rolePermissions[user.OrgRole]

	// Could also aggregate team permissions here
	// For now, org role is primary

	return permissions
}

// CheckResourceAccess verifies if a user can access a specific resource.
func (a *RBACAuthorizer) CheckResourceAccess(ctx context.Context, user *User, resourceType string, resourceID string, action string) error {
	var permission Permission

	switch resourceType {
	case "run":
		switch action {
		case "create":
			permission = PermissionRunCreate
		case "read":
			permission = PermissionRunRead
		case "update":
			permission = PermissionRunUpdate
		case "delete":
			permission = PermissionRunDelete
		}
	case "baseline":
		permission = PermissionBaselineManage
	case "team":
		permission = PermissionTeamManage
	case "org":
		permission = PermissionOrgManage
	}

	if !a.HasPermission(ctx, user, permission, resourceID) {
		return ErrPermissionDenied
	}

	return nil
}

// Helper functions

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func hashPassword(password string) string {
	// In production, use bcrypt or argon2
	h := sha256.New()
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func hashAPIKey(key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// Ensure LocalAuthProvider implements AuthProvider.
var _ AuthProvider = (*LocalAuthProvider)(nil)

// Ensure RBACAuthorizer implements Authorizer.
var _ Authorizer = (*RBACAuthorizer)(nil)
