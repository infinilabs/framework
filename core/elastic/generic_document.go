package elastic

import (
	"strconv"

	"infini.sh/framework/core/util"
)

type DocumentWithMeta[T any] struct {
	Index     string                   `json:"_index,omitempty"`
	Type      string                   `json:"_type,omitempty"`
	ID        string                   `json:"_id,omitempty"`
	Routing   string                   `json:"_routing,omitempty"`
	Score     float32                  `json:"_score,omitempty"`
	Source    T                        `json:"_source,omitempty"`
	Highlight map[string][]interface{} `json:"highlight,omitempty"`
}

type SearchResponseWithMeta[T any] struct {
	ResponseBase
	Took         int                            `json:"took,omitempty"`
	TimedOut     bool                           `json:"timed_out,omitempty"`
	Hits         SearchHits[T]                  `json:"hits,omitempty"`
	Aggregations map[string]AggregationResponse `json:"aggregations,omitempty"`
}

type SearchHits[T any] struct {
	Total    interface{}           `json:"total,omitempty"`
	MaxScore float32               `json:"max_score,omitempty"`
	Hits     []DocumentWithMeta[T] `json:"hits,omitempty"`
}

func NewGeneralTotal(total int64) util.MapStr {
	return util.MapStr{
		"value":    total,
		"relation": "eq",
	}
}

func (response *SearchResponseWithMeta[T]) GetTotal() int64 {
	if response == nil || response.Hits.Total == nil {
		return -1
	}

	switch v := response.Hits.Total.(type) {
	case map[string]interface{}:
		if val, ok := v["value"]; ok {
			return util.GetInt64Value(val)
		}
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	default:
		return util.GetInt64Value(v)
	}

	return -1
}
