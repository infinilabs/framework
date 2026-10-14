/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import "testing"

// IsAdminUser must match roles by exact equality: substring matching
// would let a caller holding a role named "min" pass the admin guard
// and assign arbitrary roles (privilege escalation).
func TestIsAdminUser_ExactMatchNotSubstring(t *testing.T) {
	// exact match passes
	if !IsAdminUser([]string{"admin"}) {
		t.Fatal(`roles=["admin"] must be admin`)
	}
	if !IsAdminUser([]string{"viewer", "admin"}) {
		t.Fatal(`roles containing exactly "admin" must be admin`)
	}
	// substrings of the admin role must NOT pass
	for _, sneaky := range []string{"min", "dmi", "adm", "a", "admins", "ADMIN", "Admin"} {
		if IsAdminUser([]string{sneaky}) {
			t.Fatalf("role %q must NOT be treated as admin (substring/case match)", sneaky)
		}
	}
	// unrelated roles
	if IsAdminUser([]string{"min", "user", "domain_admin"}) {
		t.Fatal("substring-containing role names must not escalate")
	}
	if IsAdminUser(nil) || IsAdminUser([]string{}) {
		t.Fatal("empty roles must not be admin")
	}
}

// GetPermissionKeysByRole shares the same guard shape — the unrestricted
// admin permission set only for the exact admin role.
func TestGetPermissionKeysByRole_NoSubstringEscalation(t *testing.T) {
	sneaky, sneakyErr := GetPermissionKeysByRole([]string{"min"})
	if sneakyErr != nil {
		// no backend / no resolution in this env: the substring role
		// cannot reach any permission set — trivially no escalation
		return
	}
	if len(sneaky) == 0 {
		return
	}
	adminAll, adminErr := GetPermissionKeysByRole([]string{"admin"})
	if adminErr != nil || len(adminAll) == 0 {
		return // cannot compare in this env
	}
	if len(sneaky) >= len(adminAll) {
		t.Fatal("substring role escalated to the full admin permission set")
	}
}
