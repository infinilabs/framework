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
	"context"
	"fmt"
	"net/http"
	"time"
)

const ctxUserKey = "X-INFINI-SESSION-USER"

func AddUserToContext(ctx context.Context, clam *UserSessionInfo) context.Context {
	if clam.LastLogin.Timestamp == nil {
		t := time.Now()
		clam.LastLogin.Timestamp = &t
	}
	return context.WithValue(ctx, ctxUserKey, clam)
}

func MustGetUserFromRequest(req *http.Request) *UserSessionInfo {
	reqUser, err := GetUserFromRequest(req)
	if reqUser == nil || err != nil {
		panic(err)
	}
	return reqUser
}

func GetUserFromRequest(req *http.Request) (*UserSessionInfo, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}
	return GetUserFromContext(req.Context())
}

func MustGetUserFromContext(ctx context.Context) *UserSessionInfo {
	user, err := GetUserFromContext(ctx)
	if user == nil || err != nil {
		panic(err)
	}
	return user
}

func GetUserFromContext(ctx context.Context) (*UserSessionInfo, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ctx is nil")
	}

	ctxUser := ctx.Value(ctxUserKey)
	if ctxUser == nil {
		return nil, fmt.Errorf("user not found")
	}
	reqUser, ok := ctxUser.(*UserSessionInfo)
	if !ok {
		return nil, fmt.Errorf("invalid context user")
	}
	return reqUser, nil
}
