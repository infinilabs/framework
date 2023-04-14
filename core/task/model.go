/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package task

import (
	"time"

	"infini.sh/framework/core/orm"
)

type Task struct {
	orm.ORMObjectBase

	ParentId          []string   `json:"parent_id,omitempty" elastic_mapping:"parent_id: { type: keyword }"`
	StartTimeInMillis int64      `json:"start_time_in_millis" elastic_mapping:"start_time_in_millis: { type: long }"`
	Cancellable       bool       `json:"cancellable" elastic_mapping:"cancellable: { type: boolean }"`
	Runnable          bool       `json:"runnable" elastic_mapping:"runnable: { type: boolean }"`
	Metadata          Metadata   `json:"metadata" elastic_mapping:"metadata: { type: object }"`
	Status            string     `json:"status"  elastic_mapping:"status: { type: keyword }"`
	Description       string     `json:"description,omitempty" elastic_mapping:"description: { type: text }"`
	ConfigString      string     `json:"config_string" elastic_mapping:"config_string:{ type: text }"`
	CompletedTime     *time.Time `json:"completed_time,omitempty" elastic_mapping:"completed_time: { type: date }"`
	RetryTimes        int        `json:"retry_times,omitempty" elastic_mapping:"retry_times: { type: integer }"`
	// DEPRECATED: used by old tasks
	Config_ interface{} `json:"config,omitempty" elastic_mapping:"config:{type: object,enabled:false }"`
}

type Metadata struct {
	Type   string                 `json:"type" elastic_mapping:"type: { type: keyword }"`
	Labels map[string]interface{} `json:"labels" elastic_mapping:"labels: { type: object }"`
}

type TaskResult struct {
	Success bool   `json:"success" elastic_mapping:"success: { type: boolean }"`
	Error   string `json:"error,omitempty" elastic_mapping:"error: { type: text }"`
}

const (
	StatusRunning     = "running"
	StatusComplete    = "complete"
	StatusError       = "error"
	StatusReady       = "ready"
	StatusInit        = "init"
	StatusPendingStop = "pending_stop"
	StatusStopped     = "stopped"
)

func IsEnded(status string) bool {
	switch status {
	case StatusComplete, StatusError, StatusStopped:
		return true
	default:
		return false
	}
	return false
}
