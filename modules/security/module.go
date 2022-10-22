/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/security/native"
	napi "infini.sh/framework/modules/security/native/api"
)


type Module struct {
	Enabled           bool `config:"enabled"`
}

func (module Module) Name() string {
	return "security"
}

func (module Module) Setup() {

}

var securityInited bool
func InitSecurity() {
	if securityInited{
		return
	}
	orm.RegisterSchemaWithIndexName(rbac.Role{}, "rbac-role")
	orm.RegisterSchemaWithIndexName(rbac.User{}, "rbac-user")
	native.Init()
	napi.Init()
	securityInited=true
}

func (module Module) Start() error {

	cfg := &Module{Enabled:true}

	ok,err:=env.ParseConfig("security", &cfg)
	if ok&&err!=nil{
		panic(err)
	}

	if !cfg.Enabled {
		return nil
	}

	InitSecurity()

	return nil
}

func (module Module) Stop() error {

	return nil
}

