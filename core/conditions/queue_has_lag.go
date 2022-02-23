package conditions

import (
	"fmt"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	log "github.com/cihub/seelog"
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

		consumers,ok:=queue.GetConsumerConfigsByQueueID(cfg.Id)

		if global.Env().IsDebug{
			if ok{
				for k,v:=range consumers{
					log.Trace(k,v.Id,v.Group,v.Name)
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
			//log.Error(field," offset:",latestProduceOffset==offset,",",latestProduceOffset,",",offset)
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

		//log.Error(field," depth:",depth>maxDepth,",",depth,",",maxDepth)

		if depth>maxDepth {
			return true
		}

	}
	return false
}

func (c QueueHasLag) String() string {
	return fmt.Sprintf("queue_has_lag: %v", []string(c))
}

