/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"sync"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/otel"
)

// RecordCodec converts between a queue message payload and the *event.Event
// the per-record processor convention operates on. for_each is generic over
// payloads: the default "otel" codec decodes the otel envelope (byte-
// compatible with the agent LogEvent JSON); other payload formats register
// their own codec and select it via the codec config field.
type RecordCodec interface {
	Name() string
	Decode(data []byte) (*event.Event, error)
	Encode(rec *event.Event) ([]byte, error)
}

var (
	codecsMu sync.RWMutex
	codecs   = map[string]RecordCodec{}
)

// RegisterRecordCodec adds a payload codec under its name. Intended for
// package init(); a duplicate name panics.
func RegisterRecordCodec(c RecordCodec) {
	codecsMu.Lock()
	defer codecsMu.Unlock()
	if _, exists := codecs[c.Name()]; exists {
		panic(fmt.Sprintf("record codec with same name already exists: %v", c.Name()))
	}
	codecs[c.Name()] = c
}

// GetRecordCodec returns the named codec, falling back to the default.
func GetRecordCodec(name string) RecordCodec {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	if c, ok := codecs[name]; ok {
		return c
	}
	return defaultCodec
}

// otelCodec is the default: the otel envelope JSON shared with the agent
// LogEvent format and the OTLP transport boundary.
type otelCodec struct{}

func (otelCodec) Name() string { return "otel" }
func (otelCodec) Decode(data []byte) (*event.Event, error) {
	return otel.DecodeEnvelope(data)
}
func (otelCodec) Encode(rec *event.Event) ([]byte, error) {
	return otel.EncodeEnvelope(rec)
}

var defaultCodec RecordCodec = otelCodec{}

func init() { RegisterRecordCodec(otelCodec{}) }
