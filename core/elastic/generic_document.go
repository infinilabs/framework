package elastic

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
	Took     int  `json:"took,omitempty"`
	TimedOut bool `json:"timed_out,omitempty"`
	Hits     struct {
		Total    interface{}           `json:"total,omitempty"`
		MaxScore float32               `json:"max_score,omitempty"`
		Hits     []DocumentWithMeta[T] `json:"hits,omitempty"`
	} `json:"hits,omitempty"`
	Aggregations map[string]AggregationResponse `json:"aggregations,omitempty"`
}
