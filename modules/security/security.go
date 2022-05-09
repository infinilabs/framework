/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security/rbac"
	"infini.sh/framework/modules/security/rbac/api"
	"infini.sh/framework/modules/security/rbac/permission"
)

type SecurityModule struct {
}
func (module *SecurityModule) Name() string {
	return "Security"
}

func (module *SecurityModule) Setup(cfg *config.Config) {
	orm.RegisterSchemaWithIndexName(rbac.Role{}, "rbac-role")
	orm.RegisterSchemaWithIndexName(rbac.User{}, "rbac-user")
	permission.Init()
	api.Init()
}
func (module *SecurityModule) Start() error {
	return nil
}

func (module *SecurityModule) Stop() error {

	return nil
}


