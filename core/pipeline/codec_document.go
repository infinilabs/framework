/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// documentCodec decodes queue payloads that are not otel envelopes — the
// shape external sources produce (e.g. a Kafka-native stream consuming a
// third-party topic): a JSON object becomes the record's fields wholesale;
// anything else is treated as one plain-text line and lands in the message
// field. Selected via for_each's codec config ("document").
type documentCodec struct{}

func (documentCodec) Name() string      { return "document" }
func (documentCodec) Namespace() string { return "" } // generic — same family as the otel envelope

func (documentCodec) Decode(data []byte) (*event.Event, error) {
	e := &event.Event{Fields: util.MapStr{}}
	var raw util.MapStr
	if err := util.FromJSONBytes(data, &raw); err == nil && len(raw) > 0 {
		e.Fields = raw
	} else {
		e.Fields["message"] = string(data)
	}
	// hydrate the event timestamp from the document's @timestamp:
	// Event.GetValue("@timestamp") reads Event.Timestamp, so without this
	// record processors would see ingestion time instead of event time
	e.Timestamp = time.Now().UTC()
	if ts, ok := e.Fields["@timestamp"].(string); ok && ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			e.Timestamp = t
		}
	}
	// stamp the codec hint so downstream encoding round-trips
	// symmetrically (queue outputs resolve the codec from this hint when
	// none is configured); it lives in metadata, so the encoded payload
	// stays free of framework state
	if e.Metadata.Labels == nil {
		e.Metadata.Labels = util.MapStr{}
	}
	e.Metadata.Labels[CodecLabel] = "document"
	return e, nil
}

func (documentCodec) Encode(rec *event.Event) ([]byte, error) {
	if rec.Fields == nil {
		rec.Fields = util.MapStr{}
	}
	// mirror the event timestamp back into the document: processors edit
	// the time via PutValue("@timestamp", ...) which writes Event.Timestamp
	// only — serializing Fields alone would silently drop those edits. The
	// decoder hydrates Timestamp from this same field, so unedited
	// round-trips write the value the document carried in.
	if !rec.Timestamp.IsZero() {
		rec.Fields["@timestamp"] = rec.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return util.ToJSONBytes(rec.Fields)
}

func init() { MustRegisterRecordCodec(documentCodec{}) }
