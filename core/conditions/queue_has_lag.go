package conditions

import (
	"fmt"
	"infini.sh/framework/core/queue"
)

type QueueHasLag []string

func NewDiskQueueHasLagCondition(queueName []string) (QueueHasLag) {
	return QueueHasLag(queueName)
}

func (c QueueHasLag) Check(event ValuesMap) bool {
	for _, field := range c {
		depth:=queue.Depth(field)
		if depth>0 {
			return true
		}
	}
	return false
}

func (c QueueHasLag) String() string {
	return fmt.Sprintf("queue_has_lag: %v", []string(c))
}

