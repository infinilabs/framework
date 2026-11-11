// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

// Package lane_bulk provides the in-memory counterpart of the
// indexing_merge + bulk_indexing pair: merged bulk payloads travel a lane
// (zero serialization, bounded memory, optional spill-to-disk degradation)
// instead of round-tripping the output disk queue.
//
//	merge_to_bulk_lane   input disk queue → merged bulk records → lane
//	bulk_indexing_lane   lane → Elasticsearch bulk send
//
// The two processors are additive: the legacy queue-based pair keeps
// running unchanged; pipelines opt into the memory path per task.
package lane_bulk

import (
	"fmt"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

// PayloadField is the record field carrying one merged bulk NDJSON payload.
const PayloadField = "bulk"

// codecName names the record codec for merged bulk payloads.
const codecName = "bulk"

// bulkPayloadCodec keeps lane records and queue payloads byte-identical:
// Encode returns the wrapped bulk NDJSON as-is, so a lane configured with
// `overflow: spill:<queue>` degrades records onto a disk queue that the
// legacy bulk_indexing processor can consume directly (fast/slow paths run
// the equivalent "send to ES" step, satisfying the spill equivalence rule).
type bulkPayloadCodec struct{}

func (bulkPayloadCodec) Name() string      { return codecName }
func (bulkPayloadCodec) Namespace() string { return codecName }

func (bulkPayloadCodec) Decode(data []byte) (*event.Event, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("bulk codec: empty payload")
	}
	rec := &event.Event{
		Timestamp: time.Now().UTC(),
		Fields:    util.MapStr{PayloadField: data},
		Metadata:  event.EventMetadata{Labels: util.MapStr{pipeline.CodecLabel: codecName}},
	}
	return rec, nil
}

func (bulkPayloadCodec) Encode(rec *event.Event) ([]byte, error) {
	if rec == nil {
		return nil, fmt.Errorf("bulk codec: nil record")
	}
	switch v := rec.Fields[PayloadField].(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("bulk codec: record field [%s] is not a payload", PayloadField)
	}
}

func init() {
	pipeline.MustRegisterRecordCodec(bulkPayloadCodec{})
}

// newBulkRecord wraps one merged bulk payload as a lane record. The bytes
// must be caller-owned (not alias a reusable buffer): the record outlives
// the flush that produced it. The codec label routes spill encoding and
// keeps queue round-trips symmetric.
func newBulkRecord(bulk []byte, traceID string) *event.Event {
	rec := &event.Event{
		Timestamp: time.Now().UTC(),
		Fields:    util.MapStr{PayloadField: bulk},
		Metadata:  event.EventMetadata{Labels: util.MapStr{pipeline.CodecLabel: codecName}},
	}
	pipeline.SetTraceID(rec, traceID)
	return rec
}

// bulkPayloadOf extracts the merged bulk bytes from a lane record.
func bulkPayloadOf(rec *event.Event) ([]byte, bool) {
	if rec == nil {
		return nil, false
	}
	switch v := rec.Fields[PayloadField].(type) {
	case []byte:
		return v, len(v) > 0
	case string:
		return []byte(v), v != ""
	default:
		return nil, false
	}
}
