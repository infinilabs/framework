/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package event

import (
	"infini.sh/framework/core/util"
	"time"
)

type Activity struct {
	ID      string    `json:"id,omitempty"      elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp:{type: date }"`
	Metadata  ActivityMetadata `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Changelog interface{} `json:"changelog,omitempty" elastic_mapping:"changelog:{type: object,enabled:false }"`
	Fields    util.MapStr   `json:"payload" elastic_mapping:"payload:{type: object,enabled:false }"`
}

type ActivityMetadata struct {
	Labels util.MapStr `json:"labels,omitempty"  elastic_mapping:"labels:{type: object }"`
	Category string `json:"category,omitempty"`
	Group     string `json:"group,omitempty"`
	Name     string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	User util.MapStr `json:"user,omitempty"`
}