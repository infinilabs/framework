/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package keystore

import (
	"infini.sh/framework/core/env"
	"infini.sh/framework/modules/keystore/api"
)

func (module *KeystoreModule) Name() string {
	return "keystore"
}

func (module *KeystoreModule) Setup() {
	module.Enabled = true
	exists, err := env.ParseConfig("keystore", &module)
	if exists && err != nil {
		panic(err)
	}
	if module.Enabled {
		api.Init()
	}
}
func (module *KeystoreModule) Start() error {

	return nil
}

func (module *KeystoreModule) Stop() error {
	return nil
}

type KeystoreModule struct {
	Enabled bool `config:"enabled"`
}
