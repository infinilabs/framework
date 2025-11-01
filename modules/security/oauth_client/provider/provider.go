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

package provider

import (
	"fmt"
	"github.com/pkg/errors"
	"infini.sh/framework/core/orm"
	"net/http"
	"sync"

	"golang.org/x/oauth2"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
)

type OAuthProvider interface {
	GetProfile(ctx *orm.Context, appConfig *config.OAuthConfig, cfg *oauth2.Config, tkn *oauth2.Token) *security.UserExternalProfile
	// GetOauthConfig returns the oauth config for the provider
	GetOauthConfig() *config.OAuthConfig
}

type OAuthCallbackFunc func(w http.ResponseWriter, req *http.Request, ps httprouter.Params, profile *security.UserExternalProfile) bool

var register = map[string]OAuthProvider{}
var oauthCallbacks = map[string][]OAuthCallback{} //provider,callback, continue?, //support `*` to represent any provider
var lock = sync.RWMutex{}

type OAuthCallbackMatchFunc func(string) bool
type OAuthCallback struct {
	MatchFunc    OAuthCallbackMatchFunc
	CallbackFunc OAuthCallbackFunc
}

func RegisterOAuthProvider(name string, provider OAuthProvider) {
	lock.Lock()
	defer lock.Unlock()

	register[name] = provider
}

func UnregisterOAuthProvider(name string) {
	lock.Lock()
	defer lock.Unlock()

	delete(register, name)
}

var DefaultOauthCallbackMatchFunc = func(tag string) bool {
	return true
}

func RegisterOAuthCallback(name string, callback OAuthCallback) {
	lock.Lock()
	defer lock.Unlock()

	v, ok := oauthCallbacks[name]
	if !ok {
		v = []OAuthCallback{}
	}
	if callback.MatchFunc == nil {
		callback.MatchFunc = DefaultOauthCallbackMatchFunc
	}
	v = append(v, callback)
	oauthCallbacks[name] = v
}

func GetOAuthCallbacks(name string) []OAuthCallback {
	lock.Lock()
	defer lock.Unlock()

	callbacks := []OAuthCallback{}
	v, ok := oauthCallbacks[name]
	if ok {
		callbacks = append(callbacks, v...)
	}
	v, ok = oauthCallbacks["*"]
	if ok {
		callbacks = append(callbacks, v...)
	}
	return callbacks
}

func MustGetOAuthProvider(name string) OAuthProvider {
	lock.Lock()
	defer lock.Unlock()

	v, ok := register[name]
	if ok {
		return v
	}
	panic(errors.Errorf("invalid provider: %v", name))
}

func GetExternalUserProfileID(provider string, login string) string {
	return fmt.Sprintf("%v::%v", provider, login)
}
