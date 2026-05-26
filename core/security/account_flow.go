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

import "sync"

// AccountPasswordLoginProvider lets applications keep their own password-auth realms
// while reusing the framework-owned /account/login HTTP flow and session issuance.
type AccountPasswordLoginProvider interface {
	AuthenticateByPassword(login, password string) (*UserSessionInfo, error)
}

var accountPasswordLoginProviders = sync.Map{}

func RegisterAccountPasswordLoginProvider(name string, provider AccountPasswordLoginProvider) {
	accountPasswordLoginProviders.Store(name, provider)
}

// AuthenticateAccountPasswordLogin tries application-provided password login providers
// after the native framework account path has either not matched or not succeeded.
func AuthenticateAccountPasswordLogin(login, password string) (*UserSessionInfo, error) {
	var out *UserSessionInfo
	var lastErr error

	accountPasswordLoginProviders.Range(func(key, value any) bool {
		provider, ok := value.(AccountPasswordLoginProvider)
		if !ok {
			return true
		}

		sessionUser, err := provider.AuthenticateByPassword(login, password)
		if err != nil {
			lastErr = err
			return true
		}
		if sessionUser != nil {
			out = sessionUser
			return false
		}
		return true
	})

	if out != nil {
		return out, nil
	}
	return nil, lastErr
}

// SessionTokenResponseDecorator lets applications enrich the shared login/refresh
// response with app-specific fields while reusing the framework session pipeline.
type SessionTokenResponseDecorator func(token map[string]interface{}, user *UserSessionInfo)

var sessionTokenResponseDecorators = sync.Map{}

func RegisterSessionTokenResponseDecorator(name string, decorator SessionTokenResponseDecorator) {
	sessionTokenResponseDecorators.Store(name, decorator)
}

func applySessionTokenResponseDecorators(token map[string]interface{}, user *UserSessionInfo) {
	sessionTokenResponseDecorators.Range(func(key, value any) bool {
		decorator, ok := value.(SessionTokenResponseDecorator)
		if ok {
			decorator(token, user)
		}
		return true
	})
}
