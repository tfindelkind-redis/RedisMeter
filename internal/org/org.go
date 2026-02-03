// Package org provides organization and team management for RedisMeter.
package org

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors
var (
	ErrOrgNotFound      = errors.New("organization not found")
	ErrTeamNotFound     = errors.New("team not found")
	ErrMemberNotFound   = errors.New("member not found")
	ErrOrgNameTaken     = errors.New("organization name already taken")
	ErrTeamNameTaken    = errors.New("team name already exists in organization")
	ErrAlreadyMember    = errors.New("user is already a member")
	ErrCannotRemoveOwner = errors.New("cannot remove the last owner")
)

// Organization represents a top-level organizational unit.
type Organization struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Settings    OrgSettings       `json:"settings"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// OrgSettings holds organization-wide settings.
type OrgSettings struct {
	// Storage settings
	DefaultStorageBackend string `json:"default_storage_backend,omitempty"`
	StorageQuotaMB        int64  `json:"storage_quota_mb,omitempty"`

	// Security settings
	RequireMFA            bool `json:"require_mfa,omitempty"`
	SessionTimeoutMinutes int  `json:"session_timeout_minutes,omitempty"`
	APIKeyMaxDays         int  `json:"api_key_max_days,omitempty"`

	// Feature flags
	AllowPublicBaselines bool `json:"allow_public_baselines,omitempty"`
	AllowExternalSharing bool `json:"allow_external_sharing,omitempty"`

	// Default thresholds for regression detection
	DefaultThroughputThreshold float64 `json:"default_throughput_threshold,omitempty"`
	DefaultLatencyThreshold    float64 `json:"default_latency_threshold,omitempty"`
}

// Team represents a group within an organization.
type Team struct {
	ID          string            `json:"id"`
	OrgID       string            `json:"org_id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Settings    TeamSettings      `json:"settings"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TeamSettings holds team-specific settings.
type TeamSettings struct {
	// Baseline sharing
	ShareBaselinesWithOrg bool `json:"share_baselines_with_org,omitempty"`

	// Notifications
	NotifyOnRegression  bool     `json:"notify_on_regression,omitempty"`
	NotificationEmails  []string `json:"notification_emails,omitempty"`
	SlackWebhook        string   `json:"slack_webhook,omitempty"`

	// Access control
	AllowGuestAccess bool `json:"allow_guest_access,omitempty"`
}

// Role represents a member's role in an organization or team.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// OrgMember represents a user's membership in an organization.
type OrgMember struct {
	OrgID    string    `json:"org_id"`
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	Name     string    `json:"name,omitempty"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
	InvitedBy string   `json:"invited_by,omitempty"`
}

// TeamMember represents a user's membership in a team.
type TeamMember struct {
	TeamID   string    `json:"team_id"`
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	Name     string    `json:"name,omitempty"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Invitation represents a pending invitation to join an organization or team.
type Invitation struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	TeamID     string    `json:"team_id,omitempty"`
	Email      string    `json:"email"`
	Role       Role      `json:"role"`
	InvitedBy  string    `json:"invited_by"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	AcceptedAt time.Time `json:"accepted_at,omitempty"`
	Token      string    `json:"token"`
}

// OrgManager handles organization and team operations.
type OrgManager struct {
	mu          sync.RWMutex
	orgs        map[string]*Organization
	teams       map[string]*Team
	orgMembers  map[string][]OrgMember  // orgID -> members
	teamMembers map[string][]TeamMember // teamID -> members
	invitations map[string]*Invitation  // token -> invitation
}

// NewOrgManager creates a new organization manager.
func NewOrgManager() *OrgManager {
	return &OrgManager{
		orgs:        make(map[string]*Organization),
		teams:       make(map[string]*Team),
		orgMembers:  make(map[string][]OrgMember),
		teamMembers: make(map[string][]TeamMember),
		invitations: make(map[string]*Invitation),
	}
}

// CreateOrganization creates a new organization.
func (m *OrgManager) CreateOrganization(ctx context.Context, name, displayName, description string, ownerUserID, ownerEmail, ownerName string) (*Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if name is taken
	for _, org := range m.orgs {
		if org.Name == name {
			return nil, ErrOrgNameTaken
		}
	}

	now := time.Now()
	org := &Organization{
		ID:          generateID(),
		Name:        name,
		DisplayName: displayName,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Settings:    DefaultOrgSettings(),
	}

	m.orgs[org.ID] = org

	// Add owner as member
	m.orgMembers[org.ID] = []OrgMember{{
		OrgID:    org.ID,
		UserID:   ownerUserID,
		Email:    ownerEmail,
		Name:     ownerName,
		Role:     RoleOwner,
		JoinedAt: now,
	}}

	return org, nil
}

// DefaultOrgSettings returns default organization settings.
func DefaultOrgSettings() OrgSettings {
	return OrgSettings{
		SessionTimeoutMinutes:      60,
		APIKeyMaxDays:              365,
		AllowPublicBaselines:       false,
		AllowExternalSharing:       false,
		DefaultThroughputThreshold: 0.1,  // 10% regression threshold
		DefaultLatencyThreshold:    0.15, // 15% regression threshold
	}
}

// GetOrganization retrieves an organization by ID.
func (m *OrgManager) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	org, exists := m.orgs[orgID]
	if !exists {
		return nil, ErrOrgNotFound
	}

	return org, nil
}

// GetOrganizationByName retrieves an organization by name.
func (m *OrgManager) GetOrganizationByName(ctx context.Context, name string) (*Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, org := range m.orgs {
		if org.Name == name {
			return org, nil
		}
	}

	return nil, ErrOrgNotFound
}

// UpdateOrganization updates an organization.
func (m *OrgManager) UpdateOrganization(ctx context.Context, orgID string, updates map[string]interface{}) (*Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	org, exists := m.orgs[orgID]
	if !exists {
		return nil, ErrOrgNotFound
	}

	if displayName, ok := updates["display_name"].(string); ok {
		org.DisplayName = displayName
	}
	if description, ok := updates["description"].(string); ok {
		org.Description = description
	}
	if settings, ok := updates["settings"].(OrgSettings); ok {
		org.Settings = settings
	}

	org.UpdatedAt = time.Now()

	return org, nil
}

// DeleteOrganization deletes an organization and all its teams.
func (m *OrgManager) DeleteOrganization(ctx context.Context, orgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.orgs[orgID]; !exists {
		return ErrOrgNotFound
	}

	// Delete all teams in this org
	for id, team := range m.teams {
		if team.OrgID == orgID {
			delete(m.teams, id)
			delete(m.teamMembers, id)
		}
	}

	// Delete org members
	delete(m.orgMembers, orgID)

	// Delete invitations
	for token, inv := range m.invitations {
		if inv.OrgID == orgID {
			delete(m.invitations, token)
		}
	}

	// Delete org
	delete(m.orgs, orgID)

	return nil
}

// ListOrganizations returns all organizations for a user.
func (m *OrgManager) ListOrganizations(ctx context.Context, userID string) ([]*Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Organization

	for orgID, members := range m.orgMembers {
		for _, member := range members {
			if member.UserID == userID {
				if org, exists := m.orgs[orgID]; exists {
					result = append(result, org)
				}
				break
			}
		}
	}

	return result, nil
}

// CreateTeam creates a new team within an organization.
func (m *OrgManager) CreateTeam(ctx context.Context, orgID, name, displayName, description string) (*Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.orgs[orgID]; !exists {
		return nil, ErrOrgNotFound
	}

	// Check if team name is taken in this org
	for _, team := range m.teams {
		if team.OrgID == orgID && team.Name == name {
			return nil, ErrTeamNameTaken
		}
	}

	now := time.Now()
	team := &Team{
		ID:          generateID(),
		OrgID:       orgID,
		Name:        name,
		DisplayName: displayName,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Settings:    DefaultTeamSettings(),
	}

	m.teams[team.ID] = team
	m.teamMembers[team.ID] = []TeamMember{}

	return team, nil
}

// DefaultTeamSettings returns default team settings.
func DefaultTeamSettings() TeamSettings {
	return TeamSettings{
		ShareBaselinesWithOrg: true,
		NotifyOnRegression:    true,
		AllowGuestAccess:      false,
	}
}

// GetTeam retrieves a team by ID.
func (m *OrgManager) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil, ErrTeamNotFound
	}

	return team, nil
}

// UpdateTeam updates a team.
func (m *OrgManager) UpdateTeam(ctx context.Context, teamID string, updates map[string]interface{}) (*Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return nil, ErrTeamNotFound
	}

	if displayName, ok := updates["display_name"].(string); ok {
		team.DisplayName = displayName
	}
	if description, ok := updates["description"].(string); ok {
		team.Description = description
	}
	if settings, ok := updates["settings"].(TeamSettings); ok {
		team.Settings = settings
	}

	team.UpdatedAt = time.Now()

	return team, nil
}

// DeleteTeam deletes a team.
func (m *OrgManager) DeleteTeam(ctx context.Context, teamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.teams[teamID]; !exists {
		return ErrTeamNotFound
	}

	delete(m.teams, teamID)
	delete(m.teamMembers, teamID)

	// Delete team invitations
	for token, inv := range m.invitations {
		if inv.TeamID == teamID {
			delete(m.invitations, token)
		}
	}

	return nil
}

// ListTeams returns all teams in an organization.
func (m *OrgManager) ListTeams(ctx context.Context, orgID string) ([]*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.orgs[orgID]; !exists {
		return nil, ErrOrgNotFound
	}

	var result []*Team
	for _, team := range m.teams {
		if team.OrgID == orgID {
			result = append(result, team)
		}
	}

	return result, nil
}

// ListUserTeams returns all teams a user belongs to.
func (m *OrgManager) ListUserTeams(ctx context.Context, orgID, userID string) ([]*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Team

	for teamID, members := range m.teamMembers {
		for _, member := range members {
			if member.UserID == userID {
				if team, exists := m.teams[teamID]; exists {
					if team.OrgID == orgID {
						result = append(result, team)
					}
				}
				break
			}
		}
	}

	return result, nil
}

// AddOrgMember adds a user to an organization.
func (m *OrgManager) AddOrgMember(ctx context.Context, orgID, userID, email, name string, role Role, invitedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.orgs[orgID]; !exists {
		return ErrOrgNotFound
	}

	// Check if already a member
	for _, member := range m.orgMembers[orgID] {
		if member.UserID == userID {
			return ErrAlreadyMember
		}
	}

	m.orgMembers[orgID] = append(m.orgMembers[orgID], OrgMember{
		OrgID:     orgID,
		UserID:    userID,
		Email:     email,
		Name:      name,
		Role:      role,
		JoinedAt:  time.Now(),
		InvitedBy: invitedBy,
	})

	return nil
}

// RemoveOrgMember removes a user from an organization.
func (m *OrgManager) RemoveOrgMember(ctx context.Context, orgID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members := m.orgMembers[orgID]
	if members == nil {
		return ErrOrgNotFound
	}

	// Check if this is the last owner
	ownerCount := 0
	memberIndex := -1
	for i, member := range members {
		if member.Role == RoleOwner {
			ownerCount++
		}
		if member.UserID == userID {
			memberIndex = i
		}
	}

	if memberIndex == -1 {
		return ErrMemberNotFound
	}

	if members[memberIndex].Role == RoleOwner && ownerCount == 1 {
		return ErrCannotRemoveOwner
	}

	// Remove member
	m.orgMembers[orgID] = append(members[:memberIndex], members[memberIndex+1:]...)

	// Also remove from all teams in this org
	for teamID, team := range m.teams {
		if team.OrgID == orgID {
			teamMembers := m.teamMembers[teamID]
			for i, tm := range teamMembers {
				if tm.UserID == userID {
					m.teamMembers[teamID] = append(teamMembers[:i], teamMembers[i+1:]...)
					break
				}
			}
		}
	}

	return nil
}

// UpdateOrgMemberRole updates a member's role in an organization.
func (m *OrgManager) UpdateOrgMemberRole(ctx context.Context, orgID, userID string, newRole Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members := m.orgMembers[orgID]
	if members == nil {
		return ErrOrgNotFound
	}

	for i, member := range members {
		if member.UserID == userID {
			// Check if demoting last owner
			if member.Role == RoleOwner && newRole != RoleOwner {
				ownerCount := 0
				for _, m := range members {
					if m.Role == RoleOwner {
						ownerCount++
					}
				}
				if ownerCount == 1 {
					return ErrCannotRemoveOwner
				}
			}

			m.orgMembers[orgID][i].Role = newRole
			return nil
		}
	}

	return ErrMemberNotFound
}

// ListOrgMembers returns all members of an organization.
func (m *OrgManager) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.orgs[orgID]; !exists {
		return nil, ErrOrgNotFound
	}

	return m.orgMembers[orgID], nil
}

// GetOrgMember retrieves a specific member of an organization.
func (m *OrgManager) GetOrgMember(ctx context.Context, orgID, userID string) (*OrgMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, member := range m.orgMembers[orgID] {
		if member.UserID == userID {
			return &member, nil
		}
	}

	return nil, ErrMemberNotFound
}

// AddTeamMember adds a user to a team.
func (m *OrgManager) AddTeamMember(ctx context.Context, teamID, userID, email, name string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, exists := m.teams[teamID]
	if !exists {
		return ErrTeamNotFound
	}

	// Check if user is member of the org
	isMember := false
	for _, member := range m.orgMembers[team.OrgID] {
		if member.UserID == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return fmt.Errorf("user must be a member of the organization first")
	}

	// Check if already a team member
	for _, member := range m.teamMembers[teamID] {
		if member.UserID == userID {
			return ErrAlreadyMember
		}
	}

	m.teamMembers[teamID] = append(m.teamMembers[teamID], TeamMember{
		TeamID:   teamID,
		UserID:   userID,
		Email:    email,
		Name:     name,
		Role:     role,
		JoinedAt: time.Now(),
	})

	return nil
}

// RemoveTeamMember removes a user from a team.
func (m *OrgManager) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members := m.teamMembers[teamID]
	if members == nil {
		return ErrTeamNotFound
	}

	for i, member := range members {
		if member.UserID == userID {
			m.teamMembers[teamID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}

	return ErrMemberNotFound
}

// ListTeamMembers returns all members of a team.
func (m *OrgManager) ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.teams[teamID]; !exists {
		return nil, ErrTeamNotFound
	}

	return m.teamMembers[teamID], nil
}

// CreateInvitation creates an invitation to join an organization or team.
func (m *OrgManager) CreateInvitation(ctx context.Context, orgID, teamID, email string, role Role, invitedBy string, expiresIn time.Duration) (*Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.orgs[orgID]; !exists {
		return nil, ErrOrgNotFound
	}

	if teamID != "" {
		if _, exists := m.teams[teamID]; !exists {
			return nil, ErrTeamNotFound
		}
	}

	now := time.Now()
	token := generateToken()

	inv := &Invitation{
		ID:        generateID(),
		OrgID:     orgID,
		TeamID:    teamID,
		Email:     email,
		Role:      role,
		InvitedBy: invitedBy,
		CreatedAt: now,
		ExpiresAt: now.Add(expiresIn),
		Token:     token,
	}

	m.invitations[token] = inv

	return inv, nil
}

// AcceptInvitation accepts an invitation and adds the user to the org/team.
func (m *OrgManager) AcceptInvitation(ctx context.Context, token, userID, email, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inv, exists := m.invitations[token]
	if !exists {
		return fmt.Errorf("invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		delete(m.invitations, token)
		return fmt.Errorf("invitation has expired")
	}

	if inv.Email != email {
		return fmt.Errorf("invitation email mismatch")
	}

	// Add to organization
	m.orgMembers[inv.OrgID] = append(m.orgMembers[inv.OrgID], OrgMember{
		OrgID:     inv.OrgID,
		UserID:    userID,
		Email:     email,
		Name:      name,
		Role:      inv.Role,
		JoinedAt:  time.Now(),
		InvitedBy: inv.InvitedBy,
	})

	// Add to team if specified
	if inv.TeamID != "" {
		m.teamMembers[inv.TeamID] = append(m.teamMembers[inv.TeamID], TeamMember{
			TeamID:   inv.TeamID,
			UserID:   userID,
			Email:    email,
			Name:     name,
			Role:     inv.Role,
			JoinedAt: time.Now(),
		})
	}

	// Mark invitation as accepted
	inv.AcceptedAt = time.Now()

	return nil
}

// ListPendingInvitations returns all pending invitations for an organization.
func (m *OrgManager) ListPendingInvitations(ctx context.Context, orgID string) ([]*Invitation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Invitation
	now := time.Now()

	for _, inv := range m.invitations {
		if inv.OrgID == orgID && inv.AcceptedAt.IsZero() && inv.ExpiresAt.After(now) {
			result = append(result, inv)
		}
	}

	return result, nil
}

// RevokeInvitation revokes a pending invitation.
func (m *OrgManager) RevokeInvitation(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.invitations[token]; !exists {
		return fmt.Errorf("invitation not found")
	}

	delete(m.invitations, token)
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
	return hex.EncodeToString(b)
}
