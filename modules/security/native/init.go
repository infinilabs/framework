/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package native

import "infini.sh/framework/core/api/rbac"

func init() {
	handler := rbac.Adapter{
		User: &User{},
		Role: &Role{},
	}
	rbac.RegisterAdapter("elasticsearch", handler)
}
