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
	e.Timestamp = time.Now().UTC()
	return e, nil
}

func (documentCodec) Encode(rec *event.Event) ([]byte, error) {
	return util.ToJSONBytes(rec.Fields)
}

func init() { MustRegisterRecordCodec(documentCodec{}) }
