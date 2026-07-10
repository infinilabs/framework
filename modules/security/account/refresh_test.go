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

package account

import (
	"testing"

	"infini.sh/framework/core/security"
)

type refreshTestProvider struct{}

func (refreshTestProvider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	if id != "refresh-native-user" {
		return false, nil, nil
	}

	account := &security.UserAccount{
		Name:  "Refreshed Admin",
		Email: "refreshed@example.org",
		Roles: []string{security.RoleAdmin},
	}
	account.ID = id
	return true, account, nil
}

func (refreshTestProvider) GetUserByLogin(login string) (bool, *security.UserAccount, error) {
	return false, nil, nil
}

func (refreshTestProvider) CreateUser(name, login, password string, force bool) (*security.UserAccount, error) {
	return nil, nil
}

// External providers can keep their current session payload when refreshing.
func TestBuildRefreshedSessionKeepsExternalUser(t *testing.T) {
	reqUser := &security.UserSessionInfo{
		Provider: "sso",
		Login:    "alice@example.org",
		Roles:    []string{"viewer"},
	}
	reqUser.SetUserID("external-1")

	sessionUser, err := buildRefreshedSession(reqUser)
	if err != nil {
		t.Fatalf("build refreshed session: %v", err)
	}
	if sessionUser.Login != reqUser.Login {
		t.Fatalf("expected external login %q, got %q", reqUser.Login, sessionUser.Login)
	}
	if sessionUser.UserID != reqUser.UserID {
		t.Fatalf("expected external user id %q, got %q", reqUser.UserID, sessionUser.UserID)
	}
}

// Native refreshes should pull the latest account snapshot from the registered backend.
func TestBuildRefreshedSessionReloadsNativeAccount(t *testing.T) {
	security.RegisterAuthenticationProvider("refresh-test-provider", refreshTestProvider{})

	reqUser := &security.UserSessionInfo{
		Provider: security.DefaultNativeAuthBackend,
		Login:    "stale@example.org",
		Roles:    []string{"viewer"},
	}
	reqUser.SetUserID("refresh-native-user")

	sessionUser, err := buildRefreshedSession(reqUser)
	if err != nil {
		t.Fatalf("build refreshed session: %v", err)
	}
	if sessionUser.Login != "refreshed@example.org" {
		t.Fatalf("expected refreshed login, got %q", sessionUser.Login)
	}
	if len(sessionUser.Roles) != 1 || sessionUser.Roles[0] != security.RoleAdmin {
		t.Fatalf("expected refreshed roles, got %#v", sessionUser.Roles)
	}
}
