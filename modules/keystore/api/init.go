/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import "infini.sh/framework/core/api"

func Init() {
	handler := APIHandler{}
	api.HandleAPIMethod(api.POST, "/_framework/keystore", handler.setKeystoreValue)
	api.HandleAPIMethod(api.GET, "/_framework/keystore/:key", handler.getKeystoreValue)
}
