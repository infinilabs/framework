/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elasticsearch

import "infini.sh/framework/core/security/rbac"

func init() {
	handler := rbac.Adapter{
		User: &User{},
		Role: &Role{},
	}
	rbac.RegisterAdapter("elasticsearch", handler)
}