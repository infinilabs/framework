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
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"strings"
)

type QueueHasLag []string

func NewQueueHasLagCondition(queueName []string) (QueueHasLag) {
	return QueueHasLag(queueName)
}

func (c QueueHasLag) Check(event ValuesMap) bool {
	for _, field := range c {

		var maxDepth int64
		if strings.Contains(field,">"){
			array:=strings.Split(field,">")
			maxDepth,_=util.ToInt64(strings.TrimSpace(array[1]))
			field=strings.TrimSpace(array[0])
		}

		cfg:=queue.GetOrInitConfig(field)

		consumers,ok:=queue.GetConsumerConfigsByQueueID(cfg.ID)

		if global.Env().IsDebug{
			if ok{
				for k,v:=range consumers{
					log.Trace(k,v.ID,v.Group,v.Name)
				}
			}
			log.Trace(field,len(consumers),ok)
		}

		if ok &&len(consumers)>0{
			//check
			latestProduceOffset:=queue.LatestOffset(cfg)
			offset:=queue.GetEarlierOffsetStrByQueueID(field)
			if global.Env().IsDebug {
				log.Trace(field,", ",offset, " vs ", latestProduceOffset)
			}
			if latestProduceOffset==offset {
				return false
			}else{
				return true
			}
		}

		depth:=queue.Depth(cfg)

		if global.Env().IsDebug {
			log.Trace(field, ",depth:", depth, ",", maxDepth)
		}

		if depth>maxDepth {
			return true
		}

	}
	return false
}

func (c QueueHasLag) String() string {
	return fmt.Sprintf("queue_has_lag: %v", []string(c))
}

