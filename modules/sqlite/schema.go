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

package sqlite

import (
	"fmt"
	"sync"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
)

var tableNames = sync.Map{}

func getTableName(any interface{}) string {
	pkg, t := util.GetTypeAndPackageName(any, true)
	key := fmt.Sprintf("%s-%s", pkg, t)
	v, ok := tableNames.Load(key)
	if ok {
		return v.(string)
	}
	return t
}

func initTableName(t interface{}, tableName string) string {
	pkg, objType := util.GetTypeAndPackageName(t, true)
	key := fmt.Sprintf("%s-%s", pkg, objType)
	if tableName != "" {
		v, ok := tableNames.Load(tableName)
		if ok {
			if v == key {
				log.Warnf("duplicated schema %v, %s", tableName, key)
				return tableName
			}
			panic(errors.Errorf("table name [%s][%s] already registered!", tableName, key))
		}
	} else {
		tableName = objType
	}

	tableNames.Store(key, tableName)
	tableNames.Store(tableName, key)
	return tableName
}
