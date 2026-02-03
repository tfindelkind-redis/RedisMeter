package org

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOrgManager_CreateOrganization(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "Description", "owner-123", "owner@example.com", "Owner Name")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	if org.Name != "acme" {
		t.Errorf("expected name 'acme', got '%s'", org.Name)
	}
	if org.DisplayName != "Acme Corp" {
		t.Errorf("expected display name 'Acme Corp', got '%s'", org.DisplayName)
	}
	if org.ID == "" {
		t.Error("expected org ID to be set")
	}
}

func TestOrgManager_GetOrganization(t *testing.T) {
	mgr := NewOrgManager()

	created, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	org, err := mgr.GetOrganization(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetOrganization failed: %v", err)
	}

	if org.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, org.ID)
	}
}

func TestOrgManager_GetOrganizationNotFound(t *testing.T) {
	mgr := NewOrgManager()

	_, err := mgr.GetOrganization(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent organization")
	}
}

func TestOrgManager_UpdateOrganization(t *testing.T) {
	mgr := NewOrgManager()

	created, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	updates := map[string]interface{}{
		"display_name": "Acme Inc",
		"description":  "Updated description",
	}

	updated, err := mgr.UpdateOrganization(context.Background(), created.ID, updates)
	if err != nil {
		t.Fatalf("UpdateOrganization failed: %v", err)
	}

	if updated.DisplayName != "Acme Inc" {
		t.Errorf("expected display name 'Acme Inc', got '%s'", updated.DisplayName)
	}
	if updated.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got '%s'", updated.Description)
	}
}

func TestOrgManager_DeleteOrganization(t *testing.T) {
	mgr := NewOrgManager()

	created, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	err = mgr.DeleteOrganization(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("DeleteOrganization failed: %v", err)
	}

	_, err = mgr.GetOrganization(context.Background(), created.ID)
	if err == nil {
		t.Error("expected error for deleted organization")
	}
}

func TestOrgManager_ListOrganizations(t *testing.T) {
	mgr := NewOrgManager()

	// Create multiple organizations with unique names
	for i := 0; i < 3; i++ {
		_, err := mgr.CreateOrganization(context.Background(), fmt.Sprintf("org-%d", i), fmt.Sprintf("Org %d", i), "", "owner-123", "owner@example.com", "Owner")
		if err != nil {
			t.Fatalf("CreateOrganization failed: %v", err)
		}
	}

	orgs, err := mgr.ListOrganizations(context.Background(), "owner-123")
	if err != nil {
		t.Fatalf("ListOrganizations failed: %v", err)
	}

	if len(orgs) != 3 {
		t.Errorf("expected 3 organizations, got %d", len(orgs))
	}
}

func TestOrgManager_AddOrgMember(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	err = mgr.AddOrgMember(context.Background(), org.ID, "member-456", "member@example.com", "Member", RoleMember, "owner-123")
	if err != nil {
		t.Fatalf("AddOrgMember failed: %v", err)
	}

	members, err := mgr.ListOrgMembers(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListOrgMembers failed: %v", err)
	}

	// Should have owner + new member
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestOrgManager_UpdateOrgMemberRole(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	err = mgr.AddOrgMember(context.Background(), org.ID, "member-456", "member@example.com", "Member", RoleMember, "owner-123")
	if err != nil {
		t.Fatalf("AddOrgMember failed: %v", err)
	}

	err = mgr.UpdateOrgMemberRole(context.Background(), org.ID, "member-456", RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateOrgMemberRole failed: %v", err)
	}

	members, err := mgr.ListOrgMembers(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListOrgMembers failed: %v", err)
	}

	for _, m := range members {
		if m.UserID == "member-456" {
			if m.Role != RoleAdmin {
				t.Errorf("expected role 'admin', got '%s'", m.Role)
			}
			return
		}
	}
	t.Error("member not found")
}

func TestOrgManager_RemoveOrgMember(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	err = mgr.AddOrgMember(context.Background(), org.ID, "member-456", "member@example.com", "Member", RoleMember, "owner-123")
	if err != nil {
		t.Fatalf("AddOrgMember failed: %v", err)
	}

	err = mgr.RemoveOrgMember(context.Background(), org.ID, "member-456")
	if err != nil {
		t.Fatalf("RemoveOrgMember failed: %v", err)
	}

	members, err := mgr.ListOrgMembers(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListOrgMembers failed: %v", err)
	}

	// Should only have owner
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func TestOrgManager_CreateTeam(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	team, err := mgr.CreateTeam(context.Background(), org.ID, "engineering", "Engineering", "Engineering team")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	if team.Name != "engineering" {
		t.Errorf("expected name 'engineering', got '%s'", team.Name)
	}
	if team.OrgID != org.ID {
		t.Errorf("expected org ID '%s', got '%s'", org.ID, team.OrgID)
	}
}

func TestOrgManager_GetTeam(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	created, err := mgr.CreateTeam(context.Background(), org.ID, "engineering", "Engineering", "")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	team, err := mgr.GetTeam(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetTeam failed: %v", err)
	}

	if team.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, team.ID)
	}
}

func TestOrgManager_ListTeams(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	// Create multiple teams with unique names
	for i := 0; i < 3; i++ {
		_, err := mgr.CreateTeam(context.Background(), org.ID, fmt.Sprintf("team-%d", i), fmt.Sprintf("Team %d", i), "")
		if err != nil {
			t.Fatalf("CreateTeam failed: %v", err)
		}
	}

	teams, err := mgr.ListTeams(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListTeams failed: %v", err)
	}

	if len(teams) != 3 {
		t.Errorf("expected 3 teams, got %d", len(teams))
	}
}

func TestOrgManager_DeleteTeam(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	team, err := mgr.CreateTeam(context.Background(), org.ID, "engineering", "Engineering", "")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	err = mgr.DeleteTeam(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("DeleteTeam failed: %v", err)
	}

	_, err = mgr.GetTeam(context.Background(), team.ID)
	if err == nil {
		t.Error("expected error for deleted team")
	}
}

func TestOrgManager_AddTeamMember(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	team, err := mgr.CreateTeam(context.Background(), org.ID, "engineering", "Engineering", "")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	// First add user as org member
	err = mgr.AddOrgMember(context.Background(), org.ID, "member-456", "member@example.com", "Member", RoleMember, "owner-123")
	if err != nil {
		t.Fatalf("AddOrgMember failed: %v", err)
	}

	err = mgr.AddTeamMember(context.Background(), team.ID, "member-456", "member@example.com", "Member", RoleMember)
	if err != nil {
		t.Fatalf("AddTeamMember failed: %v", err)
	}

	members, err := mgr.ListTeamMembers(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("ListTeamMembers failed: %v", err)
	}

	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func TestOrgManager_RemoveTeamMember(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	team, err := mgr.CreateTeam(context.Background(), org.ID, "engineering", "Engineering", "")
	if err != nil {
		t.Fatalf("CreateTeam failed: %v", err)
	}

	// First add user as org member
	err = mgr.AddOrgMember(context.Background(), org.ID, "member-456", "member@example.com", "Member", RoleMember, "owner-123")
	if err != nil {
		t.Fatalf("AddOrgMember failed: %v", err)
	}

	err = mgr.AddTeamMember(context.Background(), team.ID, "member-456", "member@example.com", "Member", RoleMember)
	if err != nil {
		t.Fatalf("AddTeamMember failed: %v", err)
	}

	err = mgr.RemoveTeamMember(context.Background(), team.ID, "member-456")
	if err != nil {
		t.Fatalf("RemoveTeamMember failed: %v", err)
	}

	members, err := mgr.ListTeamMembers(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("ListTeamMembers failed: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}
}

func TestOrgManager_CreateInvitation(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	inv, err := mgr.CreateInvitation(context.Background(), org.ID, "", "new@example.com", RoleMember, "owner-123", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	if inv.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got '%s'", inv.Email)
	}
	if inv.Role != RoleMember {
		t.Errorf("expected role 'member', got '%s'", inv.Role)
	}
	if inv.Token == "" {
		t.Error("expected invitation token to be set")
	}
}

func TestOrgManager_AcceptInvitation(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	inv, err := mgr.CreateInvitation(context.Background(), org.ID, "", "new@example.com", RoleMember, "owner-123", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	err = mgr.AcceptInvitation(context.Background(), inv.Token, "new-user-123", "new@example.com", "New User")
	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

	// Check that user is now a member
	members, err := mgr.ListOrgMembers(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListOrgMembers failed: %v", err)
	}

	found := false
	for _, m := range members {
		if m.UserID == "new-user-123" {
			found = true
			if m.Role != RoleMember {
				t.Errorf("expected role 'member', got '%s'", m.Role)
			}
			break
		}
	}

	if !found {
		t.Error("new user not found in org members")
	}
}

func TestOrgManager_ExpiredInvitation(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	inv, err := mgr.CreateInvitation(context.Background(), org.ID, "", "new@example.com", RoleMember, "owner-123", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	// Expire the invitation
	mgr.mu.Lock()
	if i, ok := mgr.invitations[inv.Token]; ok {
		i.ExpiresAt = time.Now().Add(-time.Hour)
	}
	mgr.mu.Unlock()

	err = mgr.AcceptInvitation(context.Background(), inv.Token, "new-user-123", "new@example.com", "New User")
	if err == nil {
		t.Error("expected error for expired invitation")
	}
}

func TestOrgManager_CancelInvitation(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	inv, err := mgr.CreateInvitation(context.Background(), org.ID, "", "new@example.com", RoleMember, "owner-123", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	err = mgr.RevokeInvitation(context.Background(), inv.Token)
	if err != nil {
		t.Fatalf("RevokeInvitation failed: %v", err)
	}

	err = mgr.AcceptInvitation(context.Background(), inv.Token, "new-user-123", "new@example.com", "New User")
	if err == nil {
		t.Error("expected error for cancelled invitation")
	}
}

func TestOrgManager_ListPendingInvitations(t *testing.T) {
	mgr := NewOrgManager()

	org, err := mgr.CreateOrganization(context.Background(), "acme", "Acme Corp", "", "owner-123", "owner@example.com", "Owner")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	// Create multiple invitations
	for i := 0; i < 3; i++ {
		_, err := mgr.CreateInvitation(context.Background(), org.ID, "", "user@example.com", RoleMember, "owner-123", 24*time.Hour)
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}
	}

	invitations, err := mgr.ListPendingInvitations(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("ListPendingInvitations failed: %v", err)
	}

	if len(invitations) != 3 {
		t.Errorf("expected 3 invitations, got %d", len(invitations))
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

func TestOrgSettings_Defaults(t *testing.T) {
	settings := DefaultOrgSettings()

	if settings.SessionTimeoutMinutes != 60 {
		t.Errorf("expected session timeout 60, got %d", settings.SessionTimeoutMinutes)
	}
	if settings.DefaultThroughputThreshold != 0.1 {
		t.Errorf("expected throughput threshold 0.1, got %f", settings.DefaultThroughputThreshold)
	}
}

func TestTeamSettings_Defaults(t *testing.T) {
	settings := DefaultTeamSettings()

	if !settings.ShareBaselinesWithOrg {
		t.Error("expected ShareBaselinesWithOrg to be true")
	}
	if !settings.NotifyOnRegression {
		t.Error("expected NotifyOnRegression to be true")
	}
}
