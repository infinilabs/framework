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

package access_token

import (
	"net/http"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"infini.sh/framework/core/credential"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

type AccessToken struct {
	orm.ORMObjectBase
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Username    string   `json:"username,omitempty"`
	Value       string   `json:"value,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

func (a AccessToken) String() string {
	return util.ToJson(a, false)
}

func ValidatePermissionByAccessToken(req *http.Request) error {
	token := strings.TrimSpace(req.Header.Get(model.API_TOKEN))
	if token == "" {
		return nil
	}
	tokenObject, err := GetByToken(token)
	if err != nil {
		return errors.NewWithHTTPCode(http.StatusUnauthorized, "invalid access token")
	}
	reqTokenPermissions := req.URL.Query()["permission"]
	if len(reqTokenPermissions) == 0 {
		return nil
	}
	userPermissionsSet := hashset.New()
	for _, item := range tokenObject.Permissions {
		userPermissionsSet.Add(item)
	}
	for _, permission := range reqTokenPermissions {
		if !userPermissionsSet.Contains(permission) {
			return errors.NewWithHTTPCode(http.StatusUnauthorized, "invalid access token permissions")
		}
	}
	return nil
}

func GetByToken(token string) (*AccessToken, error) {
	err, result := orm.GetBy("type", credential.AccessToken, credential.Credential{})
	if err != nil {
		return nil, err
	}
	for _, item := range result.Result {
		cred := credential.Credential{}
		err := util.FromJSONBytes(util.MustToJSONBytes(item), &cred)
		if err != nil {
			return nil, err
		}
		payload, err := cred.DecodeAccessToken()
		if err != nil {
			return nil, err
		}
		if payload.Value.String() == token {
			return &AccessToken{
				ORMObjectBase: cred.ORMObjectBase,
				Name:          cred.Name,
				Description:   payload.Description,
				Username:      payload.Username,
				Value:         payload.Value.String(),
				Permissions:   payload.Permissions,
			}, nil
		}
	}
	return nil, errors.NewWithHTTPCode(http.StatusNotFound, "access token not found")
}

func AddPermissionFilterByAccessToken(base []string, req *http.Request) []string {
	token := strings.TrimSpace(req.Header.Get(model.API_TOKEN))
	if token == "" {
		return base
	}
	tokenObject, err := GetByToken(token)
	if err != nil {
		log.Error("error on get access token,", err)
		return base
	}
	if len(tokenObject.Permissions) == 0 {
		return base
	}
	set := hashset.New()
	for _, item := range base {
		set.Add(item)
	}
	for _, item := range tokenObject.Permissions {
		set.Add(item)
	}
	values := make([]string, 0, set.Size())
	for _, item := range set.Values() {
		if str, ok := item.(string); ok {
			values = append(values, str)
		}
	}
	return values
}
