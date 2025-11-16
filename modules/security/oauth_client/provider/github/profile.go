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

package github

import (
	"fmt"
	"infini.sh/framework/core/orm"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/security/oauth_client/provider"
)

func init() {
	provider.RegisterOAuthProvider("github", &ProfileAPI{})
}

type ProfileAPI struct {
	api.Handler
}

func safeDereference(strPtr *string) string {
	if strPtr != nil {
		return *strPtr
	}
	return ""
}

func (handler *ProfileAPI) GetProfile(ctx *orm.Context, appConfig *config.OAuthConfig, cfg *oauth2.Config, tkn *oauth2.Token) *security.UserExternalProfile {
	//get user info
	client := github.NewClient(cfg.Client(oauth2.NoContext, tkn))
	user, res, err := client.Users.Get(oauth2.NoContext, "")
	if err != nil || user == nil || *user.Login == "" {
		panic(fmt.Errorf("failed to fetch user info: %v,%v", err, util.MustToJSON(res)))
	}
	profile := security.UserExternalProfile{}
	login := safeDereference(user.Login)
	profile.ID = provider.GetExternalUserProfileID("github", login)
	profile.AuthProvider = "github"
	profile.Login = login
	profile.Email = safeDereference(user.Email)
	profile.Name = safeDereference(user.Name)
	profile.Avatar = safeDereference(user.AvatarURL) //TODO save to local store
	profile.Payload = user
	t := time.Now()
	profile.Created = &t
	profile.Updated = &t

	return &profile
}

func (handler *ProfileAPI) GetOauthConfig() *config.OAuthConfig {
	return nil
}
