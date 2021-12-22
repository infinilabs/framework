package conditions

import (
	"fmt"
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
		depth:=queue.Depth(cfg)
		if depth>maxDepth {
			return true
		}
	}
	return false
}

func (c QueueHasLag) String() string {
	return fmt.Sprintf("queue_has_lag: %v", []string(c))
}

