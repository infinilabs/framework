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

package elastic



type ScrollResponse struct {
	Took     int    `json:"took,omitempty"`
	ScrollId string `json:"_scroll_id,omitempty,nocopy"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Hits     struct {
		MaxScore float32       `json:"max_score,omitempty"`
		Total    int64           `json:"total,omitempty"`
		Docs     []IndexDocument `json:"hits,omitempty"`
	} `json:"hits"`
	Shards ShardResponse `json:"_shards,omitempty"`
}

type ScrollResponseV7 struct {
	Took     int    `json:"took,omitempty"`
	ScrollId string `json:"_scroll_id,omitempty,nocopy"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Hits struct {
		MaxScore float32 `json:"max_score,omitempty"`
		Total    struct {
			Value    int64    `json:"value,omitempty"`
			Relation string `json:"relation,omitempty"`
		} `json:"total,omitempty"`
		Docs []IndexDocument `json:"hits,omitempty"`
	} `json:"hits"`
	Shards ShardResponse `json:"_shards,omitempty"`
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

func (scroll *ScrollResponse) GetDocs() []IndexDocument {
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

func (scroll *ScrollResponseV7) GetDocs() []IndexDocument {
	return scroll.Hits.Docs
}

func (scroll *ScrollResponseV7) GetShardResponse() ShardResponse {
	return scroll.Shards
}

