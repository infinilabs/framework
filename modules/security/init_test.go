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
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/config"
	securitycore "infini.sh/framework/core/security"
)

// TestNativeStartupRequiresStorage rejects an enabled native realm without an ORM.
func TestNativeStartupRequiresStorage(t *testing.T) {
	module := &Module{cfg: &config.WebSecurityConfig{Enabled: true, Authentication: config.AuthenticationConfig{Native: config.RealmConfig{Enabled: true}}}}
	require.ErrorContains(t, module.Start(), "ORM is not ready")
	module.cfg.Authentication.Native.Enabled = false
	require.NoError(t, module.Start())
}

// TestStaticStartup publishes configured static accounts during startup.
func TestStaticStartup(t *testing.T) {
	module := &Module{cfg: &config.WebSecurityConfig{Enabled: true, Authentication: config.AuthenticationConfig{
		Static: config.StaticAuthenticationConfig{Enabled: true, Users: []config.StaticAuthUserConfig{{ID: "startup-id", Login: "startup-login"}}},
	}}}
	require.NoError(t, module.Start())
	provider, enabled := securitycore.GetAuthenticationProvider(securitycore.StaticAuthBackend)
	require.True(t, enabled)
	found, _, err := provider.GetUserByID("startup-id")
	require.NoError(t, err)
	require.True(t, found)
	module.cfg.Enabled = false
	require.NoError(t, module.Start())
}
