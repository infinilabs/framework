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

package security

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

// Framework-issued tokens need to remain readable by console clients until the two
// stacks finish converging on a single session claim format.
func TestUserClaimsMarshalIncludesLegacyConsoleAliases(t *testing.T) {
	claims := UserClaims{
		RegisteredClaims: &jwt.RegisteredClaims{},
		UserSessionInfo: &UserSessionInfo{
			Provider: "native",
			Login:    "admin@example.org",
			Roles:    []string{RoleAdmin},
			UserID:   "user-1",
		},
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	text := string(payload)
	for _, expected := range []string{
		`"login":"admin@example.org"`,
		`"username":"admin@example.org"`,
		`"userid":"user-1"`,
		`"user_id":"user-1"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %s in %s", expected, text)
		}
	}
}

// Older console tokens only carried username/user_id, so the framework parser must
// backfill its native login/userid fields from those aliases during migration.
func TestUserClaimsUnmarshalAcceptsLegacyConsoleAliases(t *testing.T) {
	var claims UserClaims
	err := json.Unmarshal([]byte(`{
		"provider":"native",
		"username":"admin@example.org",
		"user_id":"user-1",
		"roles":["admin"]
	}`), &claims)
	if err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}

	if claims.Login != "admin@example.org" {
		t.Fatalf("expected login to be backfilled from username, got %q", claims.Login)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("expected user id to be backfilled from user_id, got %q", claims.UserID)
	}
	if claims.Provider != "native" {
		t.Fatalf("expected provider to be preserved, got %q", claims.Provider)
	}
}
