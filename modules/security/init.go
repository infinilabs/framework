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
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/module"
	_ "infini.sh/framework/modules/security/http_filters"
	"infini.sh/framework/modules/security/oauth_client"
	_ "infini.sh/framework/modules/security/orm_hooks"
	"infini.sh/framework/modules/security/rbac"
	_ "infini.sh/framework/modules/security/share"
)

type EnterpriseWebConfig struct {
}

type Module struct {
	cfg *config.WebSecurityConfig
}

func (module *Module) Name() string {
	return "security"
}

func (module *Module) Setup() {
	module.cfg = &config.WebSecurityConfig{
		Enabled: true,
		Authentication: config.AuthenticationConfig{Native: config.RealmConfig{
			Enabled: true,
		},
		},
	}

	ok, err := env.ParseConfig("web.security", &module.cfg)
	if ok && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	if !module.cfg.Enabled {
		return
	}

	if module.cfg.Authentication.Native.Enabled {
		rbac.Init()
	}

	apiHandler := oauth_client.APIHandler{}
	apiHandler.Init(module.cfg.Authentication.OAuth)

}

func (module *Module) Start() error {
	if !module.cfg.Enabled {
		return nil
	}

	return nil
}

func (module *Module) Stop() error {

	return nil
}

func init() {
	module.RegisterUserPlugin(&Module{})
}
