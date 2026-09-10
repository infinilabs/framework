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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/security"
)

func Init() {
	handler := APIHandler{}
	// keystore read/write consumed via the reverse channel (e.g. manager
	// pushing credentials); served on the web port behind login + RBAC
	api.HandleUIMethod(api.POST, "/keystore", handler.setKeystoreValue, api.RequireLogin(), api.RequirePermission(security.PermissionSystemKeystoreUpdate))
	api.HandleUIMethod(api.GET, "/keystore", handler.listKeystoreKeys, api.RequireLogin(), api.RequirePermission(security.PermissionSystemKeystoreRead))
	api.HandleAPIMethod(api.DELETE, "/keystore", handler.deleteKeystoreKey)
}
