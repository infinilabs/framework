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
	"context"
	"encoding/base64"
	"fmt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/security/http_filters"
	"math/rand"
	"net/http"

	log "github.com/cihub/seelog"
	"golang.org/x/oauth2"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	provider2 "infini.sh/framework/modules/security/oauth_client/provider"
	_ "infini.sh/framework/modules/security/oauth_client/provider/github"
	_ "infini.sh/framework/modules/security/oauth_client/provider/google"
)

type APIHandler struct {
	api.Handler
}

func init() {

	global.RegisterFuncBeforeSetup(func() {
		h := APIHandler{}
		api.HandleUIMethod(api.GET, "/sso/login/:provider_type/:provider_id", h.AuthHandler, api.AllowPublicAccess(), api.AllowOPTIONSS(), api.Feature(http_filters.FeatureCORS))
		api.HandleUIMethod(api.GET, "/sso/callback/:provider_type/:provider_id", h.CallbackHandler, api.AllowPublicAccess(), api.AllowOPTIONSS(), api.Feature(http_filters.FeatureCORS))
	})

}

const oauthSession string = "oauth-session"

func GetOauthSessionKey() string {
	return oauthSession
}

func MustGetAuthConfig(oauthProviderType, oauthProviderID string) (*config.OAuthConfig, oauth2.Config) {
	var cfg *config.OAuthConfig
	// preference to get dynamic config with provider
	if providerAPI := provider2.MustGetOAuthProvider(oauthProviderType); providerAPI != nil {
		cfg = providerAPI.GetOauthConfig()
	}

	if global.Env().IsDebug {
		log.Info(util.ToIndentJson(global.Env().SystemConfig.WebAppConfig.Security.Authentication.OAuth))
	}

	//fallback to static config
	if cfg == nil {
		tempCfg, ok := global.Env().SystemConfig.WebAppConfig.Security.Authentication.OAuth[oauthProviderID]
		if !ok || !tempCfg.Enabled {
			panic(errors.Errorf("oauth config %v/%v not found", oauthProviderType, oauthProviderID))
		}
		cfg = &tempCfg
	}

	if cfg.ClientID == "" {
		if global.Env().SystemConfig.ClusterConfig.Name != "" {
			cfg.ClientID = global.Env().SystemConfig.ClusterConfig.Name
		} else {
			cfg.ClientID = global.Env().GetInstanceID()
		}
	}

	oAuth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthorizeUrl,
			TokenURL: cfg.TokenUrl,
		},
		RedirectURL: cfg.RedirectUrl,
		Scopes:      cfg.Scopes,
	}

	return cfg, oAuth2Config
}

func (h *APIHandler) AuthHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	b := make([]byte, 16)
	rand.Read(b)

	state := base64.URLEncoding.EncodeToString(b)

	oauthProviderType := p.MustGetParameter("provider_type")
	oauthProviderID := p.MustGetParameter("provider_id")

	requestID := h.GetParameter(r, "request_id")
	tag := h.GetParameterOrDefault(r, "tag", "")
	redirectURL := h.Get(r, "redirect_url", "")

	if global.Env().IsDebug {
		log.Infof("oauth_redirect, request_id:%v, provider_type: %v, provider_id: %v", requestID, oauthProviderType, oauthProviderID)
	}
	state = fmt.Sprintf("%v:%v", oauthProviderType, state)

	session, err := api.GetSessionStore(r, oauthSession)
	if session == nil {
		panic(errors.New("session is nil"))
	}

	session.Values["state"] = state
	session.Values["request_id"] = requestID
	session.Values["provider_type"] = oauthProviderType
	session.Values["provider_id"] = oauthProviderID
	session.Values["redirect_url"] = redirectURL
	session.Values["product"] = h.Get(r, "product", "")
	session.Values["domain"] = h.Get(r, "domain", "")
	session.Values["tag"] = tag
	err = session.Save(r, w)

	oAuthConfig, oauthCfg := MustGetAuthConfig(oauthProviderType, oauthProviderID)
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

	oauthProviderType := p.MustGetParameter("provider_type")
	oauthProviderID := p.MustGetParameter("provider_id")

	oAuthConfig, oauthCfg := MustGetAuthConfig(oauthProviderType, oauthProviderID)

	session, err := api.GetSessionStore(r, oauthSession)
	if err != nil || session == nil {
		if global.Env().IsDebug {
			log.Error("sso callback failed to get session_store, aborted")
		}
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
		if global.Env().IsDebug {
			log.Errorf("failed to sso, no state match; possible csrf OR cookies not enabled: %v vs %v",
				r.URL.Query().Get("state"), session.Values["state"])
		}
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	oAuthProvider := session.Values["provider_type"].(string)
	product := session.Values["product"].(string)
	oAuthRequestID := session.Values["request_id"].(string)
	tag := session.Values["tag"].(string)

	log.Debugf("oauth_callback, provider:%v / %v, request_id:%v", oauthProviderType, oauthProviderID, oAuthRequestID)

	if oauthProviderType != oAuthProvider {
		panic("invalid provider")
	}

	code := r.URL.Query().Get("code")

	client := api.GetHttpClient("oauth_" + oAuthProvider)
	tkn, err := oauthCfg.Exchange(context.WithValue(context.Background(), oauth2.HTTPClient, client), code)
	if err != nil {
		if global.Env().IsDebug {
			log.Error("failed to sso, there was an issue getting your token: ", err, util.MustToJSON(tkn))
		}
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	if !tkn.Valid() {
		if global.Env().IsDebug {
			log.Error("failed to sso, retrieved invalid token")
		}
		http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
		return
	}

	state1 := session.Values["state"].(string)

	payload := util.MapStr{
		"product":       product,
		"request_id":    oAuthRequestID,
		"provider_type": oauthProviderType,
		"provider_id":   oauthProviderID,
		"state":         state1,
	}

	if global.Env().IsDebug {
		log.Trace(util.ToIndentJson(payload))
	}

	ctx1 := orm.NewContextWithParent(r.Context())

	providerAPI := provider2.MustGetOAuthProvider(oauthProviderType)
	userProfile := providerAPI.GetProfile(ctx1, oAuthConfig, &oauthCfg, tkn)

	callbacks := provider2.GetOAuthCallbacks(oauthProviderType)
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
		payload["provider_type"] = oauthProviderType
		payload["provider_id"] = oauthProviderID
		payload["login"] = userProfile.Login

		url := oAuthConfig.SuccessPage + "?payload=" + util.UrlEncode(util.MustToJSON(payload))
		http.Redirect(w, r, url, 302)
		return
	}
	http.Redirect(w, r, joinError(oAuthConfig.FailedPage, err), 302)
}
