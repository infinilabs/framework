// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package rbac

import (
	"strings"
	"testing"

	"infini.sh/framework/core/security"
)

// Weak passwords should now fail as normal validation errors instead of aborting
// the request flow via panic.
func TestValidateSecurePassword(t *testing.T) {
	if err := validateSecurePassword("weak"); err == nil {
		t.Fatal("expected weak password to be rejected")
	}

	if err := validateSecurePassword("StrongPassw0rd!"); err != nil {
		t.Fatalf("expected strong password to pass validation, got %v", err)
	}
}

func TestResolveUserByLogin(t *testing.T) {
	found, user, err := resolveUserByLogin("missing@example.org", nil)
	if err != nil || found || user != nil {
		t.Fatalf("expected empty result for missing user, got found=%v user=%#v err=%v", found, user, err)
	}

	items := []security.UserAccount{{}}
	items[0].Email = "admin@example.org"
	found, user, err = resolveUserByLogin("admin@example.org", items)
	if err != nil || !found || user == nil || user.Email != "admin@example.org" {
		t.Fatalf("expected single user match, got found=%v user=%#v err=%v", found, user, err)
	}

	_, _, err = resolveUserByLogin("dup@example.org", []security.UserAccount{{}, {}})
	if err == nil || !strings.Contains(err.Error(), "multiple accounts found") {
		t.Fatalf("expected duplicate login error, got %v", err)
	}
}
