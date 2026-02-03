package auth

import (
	"context"
	"testing"
	"time"
)

func TestLocalAuthProvider_CreateUser(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	user, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("expected name 'Test User', got '%s'", user.Name)
	}
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	if !user.Active {
		t.Error("expected user to be active")
	}
}

func TestLocalAuthProvider_DuplicateEmail(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "User 1")
	if err != nil {
		t.Fatalf("First CreateUser failed: %v", err)
	}

	_, err = provider.CreateUser(context.Background(), "test@example.com", "password456", "User 2")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestLocalAuthProvider_Authenticate(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Successful authentication
	user, err := provider.Authenticate(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", user.Email)
	}
}

func TestLocalAuthProvider_Login(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	token, err := provider.Login(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if token.AccessToken == "" {
		t.Error("expected access token to be set")
	}
	if token.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected token type 'Bearer', got '%s'", token.TokenType)
	}
}

func TestLocalAuthProvider_AuthenticateInvalidPassword(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	_, err = provider.Authenticate(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Error("expected error for invalid password")
	}
}

func TestLocalAuthProvider_AuthenticateUnknownUser(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.Authenticate(context.Background(), Credentials{
		Email:    "unknown@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Error("expected error for unknown user")
	}
}

func TestLocalAuthProvider_ValidateToken(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	token, err := provider.Login(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	user, err := provider.ValidateToken(context.Background(), token.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", user.Email)
	}
}

func TestLocalAuthProvider_ValidateInvalidToken(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.ValidateToken(context.Background(), "invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestLocalAuthProvider_RefreshToken(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	token, err := provider.Login(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	newToken, err := provider.RefreshToken(context.Background(), token.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if newToken.AccessToken == "" {
		t.Error("expected new access token")
	}
}

func TestLocalAuthProvider_APIKey(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	user, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create API key
	apiKey, rawKey, err := provider.CreateAPIKey(context.Background(), user.ID, "Test Key", []string{"read"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	if rawKey == "" {
		t.Error("expected API key to be returned")
	}
	if apiKey.Name != "Test Key" {
		t.Errorf("expected name 'Test Key', got '%s'", apiKey.Name)
	}
	if len(apiKey.Scopes) != 1 || apiKey.Scopes[0] != "read" {
		t.Errorf("expected scopes ['read'], got %v", apiKey.Scopes)
	}

	// Validate API key
	validatedUser, validatedKey, err := provider.ValidateAPIKey(context.Background(), rawKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}

	if validatedUser.ID != user.ID {
		t.Errorf("expected user ID '%s', got '%s'", user.ID, validatedUser.ID)
	}
	if validatedKey.ID != apiKey.ID {
		t.Errorf("expected key ID '%s', got '%s'", apiKey.ID, validatedKey.ID)
	}
}

func TestLocalAuthProvider_RevokeAPIKey(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	user, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	apiKey, rawKey, err := provider.CreateAPIKey(context.Background(), user.ID, "Test Key", []string{"read"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Revoke API key
	err = provider.RevokeAPIKey(context.Background(), apiKey.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}

	// Should fail validation
	_, _, err = provider.ValidateAPIKey(context.Background(), rawKey)
	if err == nil {
		t.Error("expected error for revoked API key")
	}
}

func TestLocalAuthProvider_InactiveUser(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	user, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Deactivate user
	provider.mu.Lock()
	if u, ok := provider.users[user.ID]; ok {
		u.User.Active = false
	}
	provider.mu.Unlock()

	// Should fail authentication
	_, err = provider.Authenticate(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Error("expected error for inactive user")
	}
}

func TestLocalAuthProvider_Logout(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	_, err := provider.CreateUser(context.Background(), "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	token, err := provider.Login(context.Background(), Credentials{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Logout
	err = provider.Logout(context.Background(), token.AccessToken)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Token should be invalid
	_, err = provider.ValidateToken(context.Background(), token.AccessToken)
	if err == nil {
		t.Error("expected error for logged out token")
	}
}

func TestLocalAuthProvider_Metadata(t *testing.T) {
	provider := NewLocalAuthProvider(DefaultLocalAuthConfig())

	meta := provider.Metadata()
	if meta.Name != "local" {
		t.Errorf("expected name 'local', got '%s'", meta.Name)
	}
	if meta.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", meta.Version)
	}
}

func TestToken_Fields(t *testing.T) {
	now := time.Now()
	token := &Token{
		AccessToken:  "test-token",
		TokenType:    "Bearer",
		ExpiresAt:    now.Add(time.Hour),
		RefreshToken: "refresh-token",
		Scopes:       []string{"read", "write"},
	}

	if token.AccessToken != "test-token" {
		t.Error("AccessToken not set correctly")
	}
	if token.TokenType != "Bearer" {
		t.Error("TokenType not set correctly")
	}
	if len(token.Scopes) != 2 {
		t.Error("Scopes not set correctly")
	}
}

func TestAPIKey_Fields(t *testing.T) {
	now := time.Now()
	apiKey := &APIKey{
		ID:        "test-key-id",
		UserID:    "user-123",
		Name:      "Test Key",
		Prefix:    "rm_",
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
		Scopes:    []string{"read"},
		Active:    true,
	}

	if apiKey.ID != "test-key-id" {
		t.Error("ID not set correctly")
	}
	if apiKey.UserID != "user-123" {
		t.Error("UserID not set correctly")
	}
	if !apiKey.Active {
		t.Error("Active not set correctly")
	}
}

func TestUser_Fields(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:        "user-123",
		Email:     "test@example.com",
		Name:      "Test User",
		CreatedAt: now,
		UpdatedAt: now,
		Active:    true,
		OrgID:     "org-123",
		OrgRole:   RoleAdmin,
	}

	if user.ID != "user-123" {
		t.Error("ID not set correctly")
	}
	if user.OrgRole != RoleAdmin {
		t.Error("OrgRole not set correctly")
	}
}

func TestRole_Constants(t *testing.T) {
	if RoleOwner != "owner" {
		t.Error("RoleOwner constant incorrect")
	}
	if RoleAdmin != "admin" {
		t.Error("RoleAdmin constant incorrect")
	}
	if RoleMember != "member" {
		t.Error("RoleMember constant incorrect")
	}
	if RoleViewer != "viewer" {
		t.Error("RoleViewer constant incorrect")
	}
}

func TestPermission_Constants(t *testing.T) {
	if PermissionRunCreate != "run:create" {
		t.Error("PermissionRunCreate constant incorrect")
	}
	if PermissionRunRead != "run:read" {
		t.Error("PermissionRunRead constant incorrect")
	}
}
