/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package oauth

import (
	"golang.org/x/oauth2"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/modules/security/config"
)

var(
	oAuthConfig       config.OAuthConfig
	defaultOAuthRoles []rbac.UserRole
	oauthCfg          oauth2.Config
)

//func New(cfg config.OAuthConfig) *OAuthRealm {
func Init(cfg config.OAuthConfig) {

	//init oauth
	if cfg.Enabled {
		api.HandleUIMethod(api.GET, "/sso/login/", apiHandler.AuthHandler)
		api.HandleUIMethod(api.GET, "/sso/callback/", apiHandler.CallbackHandler)
		api.HandleUIMethod(api.GET, "/sso/callback/:provider", apiHandler.CallbackHandler)

		oAuthConfig = cfg
		oauthCfg = oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthorizeUrl,
				TokenURL: cfg.TokenUrl,
			},
			RedirectURL: cfg.RedirectUrl,
			Scopes:      cfg.Scopes,
		}
	}

}