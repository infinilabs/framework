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

package oauth_client

import (
	"encoding/base64"
	"fmt"
	"infini.sh/framework/core/orm"
	"math/rand"
	"net/http"
	"strings"

	log "github.com/cihub/seelog"
	"golang.org/x/oauth2"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	ccache "infini.sh/framework/lib/cache"
	provider2 "infini.sh/framework/modules/security/oauth_client/provider"
	_ "infini.sh/framework/modules/security/oauth_client/provider/github"
	_ "infini.sh/framework/modules/security/oauth_client/provider/google"
)

type APIHandler struct {
	api.Handler
	cCache      *ccache.LayeredCache
	oAuthConfig map[string]config.OAuthConfig
	oauthCfg    map[string]oauth2.Config
}

func (h *APIHandler) Init(oathConfig map[string]config.OAuthConfig) {
	h.oAuthConfig = make(map[string]config.OAuthConfig)
	h.oauthCfg = make(map[string]oauth2.Config)
	h.cCache = ccache.Layered(ccache.Configure().MaxSize(10000).ItemsToPrune(100))

	for k, cfg := range oathConfig {
		h.oAuthConfig[k] = cfg
		if cfg.ClientID == "" {
			if global.Env().SystemConfig.ClusterConfig.Name!=""{
				cfg.ClientID=global.Env().SystemConfig.ClusterConfig.Name
			}else{
				cfg.ClientID = global.Env().GetInstanceID()
			}
		}

		oauthCfg := oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthorizeUrl,
				TokenURL: cfg.TokenUrl,
			},
			RedirectURL: cfg.RedirectUrl,
			Scopes:      cfg.Scopes,
		}
		h.oauthCfg[k] = oauthCfg
	}

	api.HandleUIMethod(api.GET, "/sso/login/:provider", h.AuthHandler)
	api.HandleUIMethod(api.GET, "/sso/callback/:provider", h.CallbackHandler)
}

const oauthSession string = "oauth-session"

func GetOauthSessionKey() string {
	return oauthSession
}

func (h *APIHandler) mustGetAuthConfig(provider string) (config.OAuthConfig, oauth2.Config) {
	// preference to get dynamic config with provider
	if providerAPI := provider2.MustGetOAuthProvider(provider); providerAPI != nil {
		cfg := providerAPI.GetOauthConfig()
		var oauth2Cfg oauth2.Config
		if cfg != nil {
			oauth2Cfg = oauth2.Config{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  cfg.AuthorizeUrl,
					TokenURL: cfg.TokenUrl,
				},
				RedirectURL: cfg.RedirectUrl,
				Scopes:      cfg.Scopes,
			}
			return *cfg, oauth2Cfg
		}
		//fallback to static config
	}

	oAuthConfig, ok := global.Env().SystemConfig.WebAppConfig.Security.Authentication.OAuth[provider]
	if !ok {
		panic(errors.Errorf("oauth provider %s not found", provider))
	}

	oAuth2Config, ok := h.oauthCfg[provider]
	if !ok {
		panic(errors.Errorf("oauth provider %s not found", provider))
	}
	return oAuthConfig, oAuth2Config
}

func (h *APIHandler) AuthHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	b := make([]byte, 16)
	rand.Read(b)

	state := base64.URLEncoding.EncodeToString(b)

	oauthProviderName := p.MustGetParameter("provider")
	requestID := h.GetParameter(r, "request_id")
	providerID := h.GetParameterOrDefault(r, "provider_id", oauthProviderName)
	tag := h.GetParameterOrDefault(r, "tag", "")
	redirectURL := h.Get(r, "redirect_url", "")

	log.Tracef("oauth_redirect, provider: %v, request_id:%v, provider_id: %v", oauthProviderName, requestID, providerID)
	state = fmt.Sprintf("%v:%v", providerID, state)

	session, err := api.GetSessionStore(r, oauthSession)
	if session == nil {
		panic(errors.New("session is nil"))
	}

	session.Values["state"] = state
	session.Values["request_id"] = requestID
	session.Values["provider"] = oauthProviderName
	session.Values["redirect_url"] = redirectURL
	session.Values["product"] = h.Get(r, "product", "")
	session.Values["domain"] = h.Get(r, "domain", "")
	session.Values["provider_id"] = providerID
	session.Values["tag"] = tag
	err = session.Save(r, w)

	oAuthConfig, oauthCfg := h.mustGetAuthConfig(providerID)
	if err != nil {
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	url := oauthCfg.AuthCodeURL(state)

	h.Redirect(w, r, url)
}

func joinError(url string, err error) string {
	if err != nil {
		return url + "?err=" + util.UrlEncode(err.Error())
	}
	return url
}

func (h *APIHandler) CallbackHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	provider := p.MustGetParameter("provider")
	state := r.URL.Query().Get("state")
	//decode provider id from state
	providerID := strings.Split(state, ":")[0]

	if providerID == "" {
		providerID = provider
	}

	oAuthConfig, oauthCfg := h.mustGetAuthConfig(providerID)

	session, err := api.GetSessionStore(r, oauthSession)
	if err != nil || session == nil {
		log.Error("sso callback failed to get session_store, aborted")
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	defer func() {
		session.Options.MaxAge = -1
		err = session.Save(r, w)
		if err != nil {
			log.Error(err)
		}
	}()

	if r.URL.Query().Get("state") != session.Values["state"] {
		log.Errorf("failed to sso, no state match; possible csrf OR cookies not enabled: %v vs %v",
			r.URL.Query().Get("state"), session.Values["state"])
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	oAuthProvider := session.Values["provider"].(string)
	product := session.Values["product"].(string)
	oAuthRequestID := session.Values["request_id"].(string)
	tag := session.Values["tag"].(string)

	log.Debugf("oauth_callback, provider:%v vs %v, request_id:%v", oAuthProvider, provider, oAuthRequestID)

	if provider != oAuthProvider {
		panic("invalid provider")
	}

	code := r.URL.Query().Get("code")
	tkn, err := oauthCfg.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Error("failed to sso, there was an issue getting your token: ", err, util.MustToJSON(tkn))
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	if !tkn.Valid() {
		log.Error("failed to sso, retrieved invalid token")
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	state1 := session.Values["state"].(string)

	payload := util.MapStr{
		"product":       product,
		"request_id":    oAuthRequestID,
		"auth_provider": provider,
		"state":         state1,
		"provider_id":   providerID,
	}

	ctx := orm.NewContextWithParent(r.Context())

	providerAPI := provider2.MustGetOAuthProvider(providerID)
	userProfile := providerAPI.GetProfile(ctx, &oAuthConfig, &oauthCfg, tkn)

	callbacks := provider2.GetOAuthCallbacks(providerID)
	for _, cb := range callbacks {
		matched := cb.MatchFunc(tag)
		if matched {
			next := cb.CallbackFunc(w, r, p, userProfile)
			if !next {
				log.Debug("hit break, skip next oauth callback")
				return
			}
		}
	}

	if userProfile.ID != "" && userProfile.Login != "" {
		payload["code"] = tkn.AccessToken
		payload["request_id"] = oAuthRequestID
		payload["provider"] = provider
		payload["login"] = userProfile.Login

		url := oAuthConfig.SuccessPage + "?payload=" + util.UrlEncode(util.MustToJSON(payload))
		http.Redirect(w, r, url, 302)
		return
	}
	http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
}
