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

package google

import (
	"context"
	"encoding/json"
	"fmt"
	"infini.sh/framework/core/orm"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
	"infini.sh/framework/modules/security/oauth_client/provider"
)

func init() {
	provider.RegisterOAuthProvider("google", &ProfileAPI{})
}

type ProfileAPI struct {
	api.Handler
}

func (handler *ProfileAPI) GetProfile(ctx *orm.Context, appConfig *config.OAuthConfig, cfg *oauth2.Config, tkn *oauth2.Token) *security.UserExternalProfile {
	client := cfg.Client(context.Background(), tkn)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		panic(fmt.Errorf("failed to fetch user info: %w", err))
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("unexpected status code fetching user info: %d", resp.StatusCode))
	}

	// Parse the user info
	var userInfo struct {
		Sub           string `json:"sub"`            // Unique Google user ID
		Email         string `json:"email"`          // User's email address
		VerifiedEmail bool   `json:"email_verified"` // Whether email is verified
		Name          string `json:"name"`           // User's full name
		Picture       string `json:"picture"`        // User's profile picture URL
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		panic(fmt.Errorf("failed to parse user info: %w", err))
	}

	profile := security.UserExternalProfile{}
	profile.ID = provider.GetExternalUserProfileID("google", userInfo.Sub)
	profile.AuthProvider = "google"
	profile.Login = userInfo.Sub
	profile.Email = userInfo.Email
	profile.Name = userInfo.Name
	profile.Avatar = userInfo.Picture //TODO save to local store
	profile.Payload = userInfo
	t := time.Now()
	profile.Created = &t
	profile.Updated = &t

	return &profile
}

func (handler *ProfileAPI) GetOauthConfig() *config.OAuthConfig {
	return nil
}
