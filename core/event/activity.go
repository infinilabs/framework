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
	Metadata  ActivityMetadata `json:"metadata"`
	Fields    util.MapStr   `json:"payload" elastic_mapping:"payload:{type: object,enabled:false }"`
}

type ActivityMetadata struct {
	Labels util.MapStr `json:"labels,omitempty"`
	Category string `json:"category,omitempty"`
	Group     string `json:"group,omitempty"`
	Name     string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	User util.MapStr `json:"user,omitempty"`
}