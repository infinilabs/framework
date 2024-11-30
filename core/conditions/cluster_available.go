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

package conditions

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
)

type ClusterAvailable []string

func NewClusterAvailableCondition(names []string) (ClusterAvailable) {
	return ClusterAvailable(names)
}

func (c ClusterAvailable) Check(event ValuesMap) bool {
	for _, field := range c {
		cfg:=elastic.GetMetadata(field)
		if cfg==nil{
			return false
		}
		if global.Env().IsDebug{
			log.Tracef("checking cluster [%v] health [%v]",field,cfg.IsAvailable())
		}
		if !cfg.IsAvailable(){
			return false
		}
	}
	return true
}

func (c ClusterAvailable) String() string {
	return fmt.Sprintf("cluster_available: %v", []string(c))
}

