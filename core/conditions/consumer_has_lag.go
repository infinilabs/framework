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
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

type ConsumerHasLag struct {
	Queue string
	Group string
	Consumer string
}

func NewConsumerHasLagCondition(fields  map[string]interface{}) (c ConsumerHasLag) {
	c = ConsumerHasLag{}
	if len(fields)>0{
		d:=util.MapStr(fields)
		if d.SafetyHasKey("queue"){
			c.Queue=d["queue"].(string)
			if d.SafetyHasKey("group") && d.SafetyHasKey("name"){
				c.Group=d["group"].(string)
				c.Consumer=d["name"].(string)
			}
			return c
		}
	}
	return c
}

func (c ConsumerHasLag) Check(event ValuesMap) bool {

	if c.Queue!=""{
		qConfig, ok := queue.SmartGetConfig(c.Queue)
		if ok{
			latestProduceOffset:=queue.LatestOffset(qConfig)
			if c.Group!=""&&c.Consumer!=""{
				cConfig,ok:=queue.GetConsumerConfig(qConfig.ID,c.Group,c.Consumer)
				if ok{
					consumerOffset,err:=queue.GetOffset(qConfig,cConfig)
					if err!=nil{
						panic(err)
					}
					if !consumerOffset.Equals(latestProduceOffset){
						return true
					}
				}
			}else{
				offset:=queue.GetEarlierOffsetStrByQueueID(qConfig.ID)
				if latestProduceOffset!=offset{
					return true
				}
			}
		}
	}
	return false
}

func (c ConsumerHasLag) Name() string {
	return "consumer_has_lag"
}

func (c ConsumerHasLag) String() string {
	return fmt.Sprintf("[%v][%v] on [%v]", c.Group,c.Consumer,c.Queue)
}
