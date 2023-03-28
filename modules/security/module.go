/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/credential"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/modules/security/native"
	napi "infini.sh/framework/modules/security/native/api"
	credapi "infini.sh/framework/modules/security/credential/api"
)

type Config struct {
	Enabled           bool `config:"enabled"`
	OAuth napi.OAuthConfig `config:"oauth"`
}

type Module struct {
	cfg *Config
}

func (module *Module) Name() string {
	return "security"
}

func (module *Module) Setup() {
	module.cfg=&Config{
		Enabled:true,
		OAuth: napi.OAuthConfig{
		SuccessPage: "/#/user/sso/success",
		FailedPage: "/#/user/sso/failed",
	}}

	ok,err:=env.ParseConfig("security", &module.cfg)
	if ok&&err!=nil{
		panic(err)
	}

	if !module.cfg.Enabled{
		return
	}
	napi.Init(module.cfg.OAuth)
	credapi.Init()
}

var securityInited bool
func InitSecurity() {
	if securityInited{
		return
	}
	orm.RegisterSchemaWithIndexName(rbac.Role{}, "rbac-role")
	orm.RegisterSchemaWithIndexName(rbac.User{}, "rbac-user")
	orm.RegisterSchemaWithIndexName(credential.Credential{}, "credential")
	native.Init()

	//load role from store
	napi.LoadPermission()
	securityInited=true
}

func (module *Module) Start() error {

	if !module.cfg.Enabled {
		return nil
	}

	InitSecurity()

	return nil
}

func (module *Module) Stop() error {

	return nil
}

