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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type lookupTestProvider struct {
	AuthenticationBackend
	id, login       *UserAccount
	idErr, loginErr error
	idLookups       int
}

func (p *lookupTestProvider) GetUserByID(string) (bool, *UserAccount, error) {
	p.idLookups++
	return p.id != nil, p.id, p.idErr
}
func (p *lookupTestProvider) GetUserByLogin(string) (bool, *UserAccount, error) {
	return p.login != nil, p.login, p.loginErr
}

// TestAccountLookup covers ID and login lookup results and backend errors.
func TestAccountLookup(t *testing.T) {
	saved := map[string]AuthenticationBackend{}
	authenticationBackendBackendProviders.Range(func(k, v any) bool {
		saved[k.(string)] = v.(AuthenticationBackend)
		authenticationBackendBackendProviders.Delete(k)
		return true
	})
	t.Cleanup(func() {
		authenticationBackendBackendProviders.Range(func(k, v any) bool { authenticationBackendBackendProviders.Delete(k); return true })
		for k, v := range saved {
			RegisterAuthenticationProvider(k, v)
		}
	})
	account := &UserAccount{Email: "alice"}
	account.ID = "alice-id"
	unavailable := errors.New("storage unavailable")
	for _, tc := range []struct {
		name      string
		providers []*lookupTestProvider
		want      error
		found     bool
	}{
		{name: "no providers"},
		{name: "no matches", providers: []*lookupTestProvider{{}}, want: ErrUserNotFound},
		{name: "unique", providers: []*lookupTestProvider{{id: account, login: account}, {}}, found: true},
		{name: "backend failure", providers: []*lookupTestProvider{{idErr: unavailable, loginErr: unavailable}}, want: unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authenticationBackendBackendProviders.Range(func(k, v any) bool { authenticationBackendBackendProviders.Delete(k); return true })
			for i, p := range tc.providers {
				RegisterAuthenticationProvider(fmt.Sprint(i), p)
			}
			provider, found, got, err := ResolveUserByLogin("alice")
			require.Equal(t, tc.found, found)
			if tc.found {
				require.NoError(t, err)
				require.Equal(t, "0", provider)
				require.Same(t, account, got)
			} else {
				require.Error(t, err)
				if tc.want != nil {
					require.ErrorIs(t, err, tc.want)
				}
				require.Empty(t, provider)
				require.Nil(t, got)
			}
			found, got, err = GetUserByLogin("alice")
			require.Equal(t, tc.found, found)
			if !tc.found {
				require.Error(t, err)
				require.Nil(t, got)
			}
			for _, p := range tc.providers {
				require.Zero(t, p.idLookups, "login lookup must use the login query")
			}
			provider, got, err = GetUserByID("alice-id")
			if tc.found {
				require.NoError(t, err)
				require.Equal(t, "0", provider)
				require.Same(t, account, got)
			} else {
				require.Error(t, err)
				if tc.want != nil {
					require.ErrorIs(t, err, tc.want)
				}
				require.Empty(t, provider)
				require.Nil(t, got)
			}
		})
	}
}
