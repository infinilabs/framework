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

package staticauth

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
)

// TestStaticIdentityValidation publishes complete maps and retains them on invalid configuration.
func TestStaticIdentityValidation(t *testing.T) {
	originalByID, originalByLogin := authUsersByID, authUsersByLogin
	t.Cleanup(func() {
		mu.Lock()
		authUsersByID, authUsersByLogin = originalByID, originalByLogin
		mu.Unlock()
	})
	cfg := config.StaticAuthenticationConfig{Enabled: true, Users: []config.StaticAuthUserConfig{
		{ID: " alice ", Login: " bob ", Name: "Same", Password: "password"},
		{ID: " bob ", Login: " alice ", Name: "Same"},
		{Login: "Alice"},
	}}
	require.NoError(t, InitAuthentication(cfg, nil))
	p := &provider{}
	found, account, err := p.GetUserByID("alice")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "bob", account.Email)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(account.Password), []byte("password")))
	_, byLogin, err := p.GetUserByLogin("bob")
	require.NoError(t, err)
	require.Same(t, account, byLogin)
	found, _, err = p.GetUserByID("Alice")
	require.NoError(t, err)
	require.True(t, found)
	for _, users := range [][]config.StaticAuthUserConfig{
		{{ID: "x", Login: "one"}, {ID: " x ", Login: "two"}},
		{{Login: "one"}, {ID: "one", Login: "two"}},
		{{ID: "x", Login: "one"}, {ID: "y", Login: " one "}},
		{{Login: " "}},
		{{Login: "new", Password: strings.Repeat("x", 73)}},
	} {
		require.Error(t, InitAuthentication(config.StaticAuthenticationConfig{Enabled: true, Users: users}, nil))
		found, after, err := p.GetUserByID("alice")
		require.NoError(t, err)
		require.True(t, found)
		require.Same(t, account, after)
		found, _, err = p.GetUserByLogin("one")
		require.NoError(t, err)
		require.False(t, found)
	}
	require.NoError(t, InitAuthentication(config.StaticAuthenticationConfig{}, nil))
	found, after, err := p.GetUserByID("alice")
	require.NoError(t, err)
	require.True(t, found)
	require.Same(t, account, after)
}

type nativeIdentityTestProvider struct {
	security.AuthenticationBackend
	ids, logins     map[string]*security.UserAccount
	idErr, loginErr error
}

func (p *nativeIdentityTestProvider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	account, found := p.ids[id]
	return found, account, p.idErr
}
func (p *nativeIdentityTestProvider) GetUserByLogin(login string) (bool, *security.UserAccount, error) {
	account, found := p.logins[login]
	return found, account, p.loginErr
}

// TestNativeIdentityValidation publishes static accounts only after native checks pass.
func TestNativeIdentityValidation(t *testing.T) {
	oldID, oldLogin := authUsersByID, authUsersByLogin
	t.Cleanup(func() {
		mu.Lock()
		authUsersByID, authUsersByLogin = oldID, oldLogin
		mu.Unlock()
	})
	existing := config.StaticAuthenticationConfig{Enabled: true, Users: []config.StaticAuthUserConfig{{ID: "existing-id", Login: "existing-login"}}}
	candidate := config.StaticAuthenticationConfig{Enabled: true, Users: []config.StaticAuthUserConfig{{ID: "id", Login: "login"}}}
	user := &security.UserAccount{Email: "login"}
	user.ID = "id"
	unavailable := errors.New("storage unavailable")
	for _, tc := range []struct {
		name     string
		provider *nativeIdentityTestProvider
		want     error
	}{
		{"distinct identities", &nativeIdentityTestProvider{}, nil},
		{"duplicate ID", &nativeIdentityTestProvider{ids: map[string]*security.UserAccount{"id": user}}, security.ErrIdentityConflict},
		{"duplicate login", &nativeIdentityTestProvider{logins: map[string]*security.UserAccount{"login": user}}, security.ErrIdentityConflict},
		{"equal values across fields", &nativeIdentityTestProvider{ids: map[string]*security.UserAccount{"login": user}, logins: map[string]*security.UserAccount{"id": user}}, nil},
		{"ID lookup failure", &nativeIdentityTestProvider{idErr: unavailable}, unavailable},
		{"login lookup failure", &nativeIdentityTestProvider{loginErr: unavailable}, unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, InitAuthentication(existing, nil))
			p := &provider{}
			_, original, err := p.GetUserByID("existing-id")
			require.NoError(t, err)
			err = InitAuthentication(candidate, tc.provider)
			if tc.want == nil {
				require.NoError(t, err)
				found, account, err := p.GetUserByID("id")
				require.NoError(t, err)
				require.True(t, found)
				found, byLogin, err := p.GetUserByLogin("login")
				require.NoError(t, err)
				require.True(t, found)
				require.Same(t, account, byLogin)
				found, _, err = p.GetUserByID("existing-id")
				require.NoError(t, err)
				require.False(t, found)
			} else {
				require.ErrorIs(t, err, tc.want)
				found, after, err := p.GetUserByID("existing-id")
				require.NoError(t, err)
				require.True(t, found)
				require.Same(t, original, after)
				found, _, err = p.GetUserByID("id")
				require.NoError(t, err)
				require.False(t, found)
				found, _, err = p.GetUserByLogin("login")
				require.NoError(t, err)
				require.False(t, found)
			}
		})
	}
}
