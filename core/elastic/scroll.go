package elastic

import 	"github.com/segmentio/encoding/json"


type ScrollResponseAPI interface {
	GetScrollId() string
	SetScrollId(id string)
	GetHitsTotal() int64
	GetShardResponse() ShardResponse
	GetDocs() []json.RawMessage
}

type ScrollResponse struct {
	Took     int    `json:"took,omitempty"`
	ScrollId string `json:"_scroll_id,omitempty,nocopy"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Hits     struct {
		MaxScore float32       `json:"max_score,omitempty"`
		Total    int64           `json:"total,omitempty"`
		Docs     []json.RawMessage `json:"hits,omitempty"`
	} `json:"hits"`
	Shards ShardResponse `json:"_shards,omitempty"`
}

type ScrollResponseV7 struct {
	ScrollResponse
	Hits struct {
		MaxScore float32 `json:"max_score,omitempty"`
		Total    struct {
			Value    int64    `json:"value,omitempty"`
			Relation string `json:"relation,omitempty"`
		} `json:"total,omitempty"`
		Docs []json.RawMessage `json:"hits,omitempty"`
	} `json:"hits"`
}

func (scroll *ScrollResponse) GetHitsTotal() int64 {
	return scroll.Hits.Total
}

func (scroll *ScrollResponse) GetScrollId() string {
	return scroll.ScrollId
}

func (scroll *ScrollResponse) SetScrollId(id string) {
	scroll.ScrollId = id
}

func (scroll *ScrollResponse) GetDocs() []json.RawMessage {
	return scroll.Hits.Docs
}

func (scroll *ScrollResponse) GetShardResponse() ShardResponse {
	return scroll.Shards
}

func (scroll *ScrollResponseV7) GetHitsTotal() int64 {
	return scroll.Hits.Total.Value
}

func (scroll *ScrollResponseV7) GetScrollId() string {
	return scroll.ScrollId
}

func (scroll *ScrollResponseV7) SetScrollId(id string) {
	scroll.ScrollId = id
}

func (scroll *ScrollResponseV7) GetDocs() []json.RawMessage {
	return scroll.Hits.Docs
}

func (scroll *ScrollResponseV7) GetShardResponse() ShardResponse {
	return scroll.Shards
}

