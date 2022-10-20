/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package badger

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/filter"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/module"
	"path"
)

type Module struct {
	handler *BadgerFilter
}

func (module *Module) Name() string {
	return "Badger"
}

func (module *Module) Setup() {
	ok,err:=env.ParseConfig("badger", module)
	if ok&&err!=nil{
		panic(err)
	}

	module.handler= &BadgerFilter{
		Path : path.Join(global.Env().GetDataDir(),"badger"),
	}
	filter.Register("badger", module.handler)
	kv.Register("badger", module.handler)

}

func (module *Module) Start() error {
	if module.handler != nil {
		module.handler.Open()
	}
	return nil
}

func (module *Module) Stop() error {
	if module.handler != nil {
		module.handler.Close()
	}
	return nil
}

func init() {
	module.RegisterSystemModule(&Module{})
}