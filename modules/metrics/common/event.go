/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package common

import (
	"fmt"
	"infini.sh/framework/core/util"
	"time"
)

type MetricEvent struct {
	Agent      *AgentMeta    `json:"agent"`
	Timestamp  time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	Metadata   EventMetadata `json:"metadata"`
	MetricData util.MapStr   `json:"metric"`
}

type EventMetadata struct {
	Labels util.MapStr `json:"labels,omitempty"`
	Category string `json:"category,omitempty"`
	Name     string `json:"name,omitempty"`
	Datatype string `json:"datatype,omitempty"`
}

func (e *MetricEvent) String() string {
	return fmt.Sprintf("%v-%v,%v,%v", e.Timestamp.UTC().Unix(), e.Metadata, e.Agent.Tags, e.Agent.Labels)
}

type AgentMeta struct {
	QueueName string `json:"-"`

	AgentID  string   `json:"id,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
	MajorIP  string   `json:"ip,omitempty"`
	IP       []string `json:"binding_ip,omitempty"`

	Tags   []string          `json:"tags,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}
