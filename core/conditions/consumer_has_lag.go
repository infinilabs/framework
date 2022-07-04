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
			if d.SafetyHasKey("group") && d.SafetyHasKey("cosumer"){
				c.Group=d["group"].(string)
				c.Consumer=d["consumer"].(string)
			}
			return c
		}
	}
	return c
}

func (c ConsumerHasLag) Check(event ValuesMap) bool {

	if c.Queue!=""{
		qConfig, ok := queue.GetConfigByKey(c.Queue)
		if ok{
			latestProduceOffset:=queue.LatestOffset(qConfig)
			if c.Group!=""&&c.Consumer!=""{
				cConfig,ok:=queue.GetConsumerConfig(c.Queue,c.Group,c.Consumer)
				if ok{
					consumerOffset,err:=queue.GetOffset(qConfig,cConfig)
					if err!=nil{
						panic(err)
					}
					if consumerOffset!=latestProduceOffset{
						return true
					}
				}
			}else{
				offset:=queue.GetEarlierOffsetStrByQueueID(c.Queue)

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
