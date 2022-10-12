/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package migration

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/modules/migration/api"
)

func (module *MigrationModule) Name() string {
	return "migration"
}

func (module *MigrationModule) Setup(cfg *config.Config) {
	api.Init()
}
func (module *MigrationModule) Start() error {
	return nil
}

func (module *MigrationModule) Stop() error {
	return nil
}

type MigrationModule struct {
}
