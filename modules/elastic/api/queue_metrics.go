package api

import (
	"infini.sh/framework/modules/elastic/common"
)

func (h *APIHandler) getQueueMetrics()([]common.MetricItem, error){
	queueMetricItems := []common.MetricItem{}
	//Get Thread Pool queue
	return queueMetricItems, nil
}