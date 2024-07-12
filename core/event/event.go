/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package event

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"infini.sh/framework/core/util"
)

type Event struct {
	Agent     *AgentMeta    `json:"agent"`

	QueueName string        `json:"-"`

	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	Metadata  EventMetadata `json:"metadata"`
	Fields    util.MapStr   `json:"payload"`

	Meta       util.MapStr `json:"-"`
	Private    interface{} `json:"-"` // for beats private use
	TimeSeries bool        `json:"-"` // true if the event contains timeseries data
}

type EventMetadata struct {
	Labels   util.MapStr `json:"labels,omitempty"`
	Category string      `json:"category,omitempty"`
	Name     string      `json:"name,omitempty"`
	Version  string      `json:"version,omitempty"`
	Datatype string      `json:"datatype,omitempty"`
}

func (e *Event) String() string {
	return fmt.Sprintf("%v %v %v", e.Timestamp.UTC().Unix(), e.Metadata.Category, e.Metadata.Name)
}

type AgentMeta struct {
	DefaultMetricQueueName string `json:"-"`
	LoggingQueueName       string `json:"-"`

	AgentID  string   `json:"id,omitempty"`
	HostID   string   `json:"host_id,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
	MajorIP  string   `json:"major_ip,omitempty"`
	IP       []string `json:"ips,omitempty"`

	Tags   []string          `json:"tags,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// SetID overwrites the "id" field in the events metadata.
// If Meta is nil, a new Meta dictionary is created.
func (e *Event) SetID(id string) {
	if e.Meta == nil {
		e.Meta = util.MapStr{}
	}
	e.Meta["_id"] = id
}

func (e *Event) GetValue(key string) (interface{}, error) {
	if key == "@timestamp" {
		return e.Timestamp, nil
	} else if subKey, ok := metadataKey(key); ok {
		if subKey == "" || e.Meta == nil {
			return e.Meta, nil
		}
		return e.Meta.GetValue(subKey)
	}
	return e.Fields.GetValue(key)
}

var errNoTimestamp = errors.New("no timestamp found")
var errNoMapStr = errors.New("no data found")

func (e *Event) PutValue(key string, v interface{}) (interface{}, error) {
	if key == "@timestamp" {
		switch ts := v.(type) {
		case time.Time:
			e.Timestamp = ts
		case util.Time:
			e.Timestamp = time.Time(ts)
		default:
			return nil, errNoTimestamp
		}
		return nil, nil
	} else if subKey, ok := metadataKey(key); ok {
		if subKey == "" {
			switch meta := v.(type) {
			case util.MapStr:
				e.Meta = meta
			case map[string]interface{}:
				e.Meta = meta
			default:
				return nil, errNoMapStr
			}
		} else if e.Meta == nil {
			e.Meta = util.MapStr{}
		}
		return e.Meta.Put(subKey, v)
	}

	return e.Fields.Put(key, v)
}

func (e *Event) Delete(key string) error {
	if subKey, ok := metadataKey(key); ok {
		if subKey == "" {
			e.Meta = nil
			return nil
		}
		if e.Meta == nil {
			return nil
		}
		return e.Meta.Delete(subKey)
	}
	return e.Fields.Delete(key)
}

func metadataKey(key string) (string, bool) {
	if !strings.HasPrefix(key, "@metadata") {
		return "", false
	}

	subKey := key[len("@metadata"):]
	if subKey == "" {
		return "", true
	}
	if subKey[0] == '.' {
		return subKey[1:], true
	}
	return "", false
}

// SetErrorWithOption sets jsonErr value in the event fields according to addErrKey value.
func (e *Event) SetErrorWithOption(jsonErr util.MapStr, addErrKey bool) {
	if addErrKey {
		e.Fields["error"] = jsonErr
	}
}
