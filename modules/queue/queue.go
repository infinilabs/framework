/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"infini.sh/framework/modules/queue/common"
)

type Module struct {
}

func (this *Module) Setup() {
}
func (this *Module) Start() error {
	common.InitQueueMetadata()
	return nil
}
func (this *Module) Stop() error {
	common.PersistQueueMetadata()
	return nil
}
func (this *Module) Name() string {
	return "queue"
}
