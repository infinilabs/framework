/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package task

import (
	"infini.sh/framework/core/orm"
	"time"
)

type Task struct {
	orm.ORMObjectBase

	ParentId          []string `json:"parent_id,omitempty" elastic_mapping:"parent_id: { type: keyword }"`
	StartTimeInMillis int64  `json:"start_time_in_millis" elastic_mapping:"start_time_in_millis: { type: long }"`
	Cancellable       bool   `json:"cancellable" elastic_mapping:"cancellable: { type: boolean }"`
	Runnable          bool   `json:"runnable" elastic_mapping:"runnable: { type: boolean }"`
	Metadata Metadata `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Status string `json:"status"  elastic_mapping:"status: { type: keyword }"`
	Description string `json:"description,omitempty" elastic_mapping:"description: { type: text }"`
	Parameters map[string]interface{} `json:"parameters" elastic_mapping:"parameters:{type: object,enabled:false }"`
	CompletedTime *time.Time `json:"completed_time,omitempty" elastic_mapping:"completed_time: { type: date }"`
}

type Metadata struct {
	Type        string `json:"type" elastic_mapping:"type: { type: keyword }"`
	Labels map[string]interface{} `json:"labels" elastic_mapping:"labels: { type: object }"`
}

type Log struct {
	ID string `json:"id"  elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	TaskId string `json:"task_id"  elastic_mapping:"task_id: { type: keyword }"`
	Status string `json:"status"  elastic_mapping:"status: { type: keyword }"`
	Type   string `json:"type"  elastic_mapping:"type: { type: keyword }"`
	Action LogAction `json:"action"  elastic_mapping:"action: { type: object }"`
	Content   string `json:"content" elastic_mapping:"content: { type: text }"`
	Timestamp time.Time `json:"timestamp" elastic_mapping:"timestamp: { type: date }"`
}

type LogAction struct {
	Result *LogResult `json:"result,omitempty"  elastic_mapping:"result:{type: object}"`
	Parameters map[string]interface{} `json:"parameters" elastic_mapping:"parameters:{type: object,enabled:false }"`
}

type LogResult struct {
	Success bool   `json:"success" elastic_mapping:"success: { type: boolean }"`
	Error   string `json:"error" elastic_mapping:"error: { type: text }"`
}

const (
	StatusRunning        = "running"
	StatusComplete       = "complete"
	StatusCancel         = "cancel"
	StatusPause          = "pause"
	StatusError          = "error"
	StatusReady          = "ready"
	StatusInit			 = "init"
	StatusStopped        = "stopped"
)