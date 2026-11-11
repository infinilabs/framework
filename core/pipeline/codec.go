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
//
// Namespace reports the record family the codec produces ("document",
// "attachment", "" for generic). Record contracts (D2) match on the
// namespace, not the codec name, so v2 codecs producing the same family
// keep dependent processors source-compatible.
type RecordCodec interface {
	Name() string
	Namespace() string
	Decode(data []byte) (*event.Event, error)
	Encode(rec *event.Event) ([]byte, error)
}

// CodecLabel is the metadata label stamping the codec that decoded a
// record, so output encoding round-trips symmetrically (hosts and codecs
// stamp it at decode time).
const CodecLabel = "_codec"

// CodecHint returns the record's originating codec name ("" when unset).
func CodecHint(rec *event.Event) string {
	if rec == nil || rec.Metadata.Labels == nil {
		return ""
	}
	v, _ := rec.Metadata.Labels[CodecLabel].(string)
	return v
}

// CodecInfo describes a registered codec for discovery APIs
// (GET /pipeline/codecs).
type CodecInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Version     int64  `json:"version"`
	Description string `json:"description,omitempty"`
}

type codecEntry struct {
	codec   RecordCodec
	version int64
}

var (
	codecsMu sync.RWMutex
	codecs   = map[string]*codecEntry{}
)

// RegisterRecordCodec adds a payload codec under its name. Intended for
// package init() and runtime registration alike: unlike the historical
// panic-on-duplicate behavior it returns an error so runtime callers can
// react; init-time callers wanting the old hard-fail semantics use
// MustRegisterRecordCodec.
func RegisterRecordCodec(c RecordCodec) error {
	codecsMu.Lock()
	defer codecsMu.Unlock()
	if _, exists := codecs[c.Name()]; exists {
		return fmt.Errorf("record codec with same name already exists: %v", c.Name())
	}
	codecs[c.Name()] = &codecEntry{codec: c, version: 1}
	return nil
}

// MustRegisterRecordCodec is the init-time variant: a duplicate name is a
// programming error and panics, keeping the historical defense against two
// packages registering the same codec.
func MustRegisterRecordCodec(c RecordCodec) {
	if err := RegisterRecordCodec(c); err != nil {
		panic(err)
	}
}

// RegisterRecordCodecOrReplace upgrades an existing registration under the
// same name (version increments); registers fresh otherwise. For dev/test
// iteration and controlled upgrades — running pipelines keep their compiled
// reference (registration is dynamic, binding is compile-time), a pipeline
// reload/rebuild picks up the replacement.
func RegisterRecordCodecOrReplace(c RecordCodec) error {
	if c.Name() == "" {
		return fmt.Errorf("record codec name is empty")
	}
	codecsMu.Lock()
	defer codecsMu.Unlock()
	version := int64(1)
	if prev, exists := codecs[c.Name()]; exists {
		version = prev.version + 1
	}
	codecs[c.Name()] = &codecEntry{codec: c, version: version}
	return nil
}

// GetRecordCodec returns the named codec, falling back to the default.
func GetRecordCodec(name string) RecordCodec {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	if e, ok := codecs[name]; ok {
		return e.codec
	}
	return defaultCodec
}

// ListRecordCodecs snapshots the registry for discovery.
func ListRecordCodecs() []CodecInfo {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	out := make([]CodecInfo, 0, len(codecs))
	for _, e := range codecs {
		out = append(out, CodecInfo{
			Name:      e.codec.Name(),
			Namespace: e.codec.Namespace(),
			Version:   e.version,
		})
	}
	return out
}

// otelCodec is the default: the otel envelope JSON shared with the agent
// LogEvent format and the OTLP transport boundary.
type otelCodec struct{}

func (otelCodec) Name() string      { return "otel" }
func (otelCodec) Namespace() string { return "" }
func (otelCodec) Decode(data []byte) (*event.Event, error) {
	return otel.DecodeEnvelope(data)
}
func (otelCodec) Encode(rec *event.Event) ([]byte, error) {
	return otel.EncodeEnvelope(rec)
}

var defaultCodec RecordCodec = otelCodec{}

func init() { MustRegisterRecordCodec(otelCodec{}) }
