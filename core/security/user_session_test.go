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
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestUserClaimsMarshalUsesFrameworkFields(t *testing.T) {
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

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal claims json: %v", err)
	}

	if data["login"] != "admin@example.org" {
		t.Fatalf("expected login field, got %#v", data["login"])
	}
	if data["userid"] != "user-1" {
		t.Fatalf("expected userid field, got %#v", data["userid"])
	}
	if _, exists := data["username"]; exists {
		t.Fatalf("did not expect legacy username alias in claims: %s", payload)
	}
	if _, exists := data["user_id"]; exists {
		t.Fatalf("did not expect legacy user_id alias in claims: %s", payload)
	}
}

func TestUserSessionInfoIsValidReturnsFalseForIncompleteUser(t *testing.T) {
	user := &UserSessionInfo{
		Provider: "native",
		Login:    "admin@example.org",
	}

	if user.IsValid() {
		t.Fatal("expected incomplete user session to be invalid")
	}
}
