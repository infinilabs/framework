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

package otlp

import (
	"testing"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/otel"
	"infini.sh/framework/core/util"
)

func TestCodecRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 15, 10, 30, 0, 123456789, time.UTC)
	events := []*event.Event{
		{
			Timestamp: ts,
			Fields: util.MapStr{
				otel.FieldLogLevel:   "ERROR",
				otel.FieldMessage:    "connection refused",
				otel.FieldTraceID:    "0af7651916cd43dd8448eb211c80319c",
				"service_name":       "payment-svc",
				"retry_count":        int64(3),
				"timeout_seconds":    1.5,
				"tls_enabled":        true,
			},
			Meta: util.MapStr{
				otel.MetaResourceKey: util.MapStr{
					otel.ResourceHostName: "web-1",
				},
			},
		},
		{
			Timestamp: ts.Add(time.Second),
			Fields: util.MapStr{
				otel.FieldLogLevel: "INFO",
				otel.FieldMessage:  "healthy",
			},
			Meta: util.MapStr{
				otel.MetaResourceKey: util.MapStr{
					otel.ResourceHostName: "web-2",
				},
			},
		},
	}

	req := BuildExportRequest(events)

	// two distinct resources => two ResourceLogs
	if len(req.GetResourceLogs()) != 2 {
		t.Fatalf("resource groups = %d, want 2", len(req.GetResourceLogs()))
	}
	total := 0
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			total += len(sl.GetLogRecords())
		}
	}
	if total != 2 {
		t.Fatalf("total records = %d, want 2", total)
	}

	decoded := RecordsFromExportRequest(req)
	if len(decoded) != 2 {
		t.Fatalf("decoded events = %d, want 2", len(decoded))
	}

	var first *event.Event
	for _, e := range decoded {
		if e.Fields[otel.FieldLogLevel] == "ERROR" {
			first = e
		}
	}
	if first == nil {
		t.Fatalf("error record lost in round trip: %+v", decoded)
	}
	if got := first.Fields[otel.FieldMessage]; got != "connection refused" {
		t.Fatalf("message = %v", got)
	}
	if got := first.Fields["service_name"]; got != "payment-svc" {
		t.Fatalf("service_name = %v", got)
	}
	if got := first.Fields["retry_count"]; got != int64(3) {
		t.Fatalf("retry_count = %v (%T)", got, got)
	}
	if got := first.Fields["timeout_seconds"]; got != 1.5 {
		t.Fatalf("timeout_seconds = %v (%T)", got, got)
	}
	if got := first.Fields["tls_enabled"]; got != true {
		t.Fatalf("tls_enabled = %v", got)
	}
	if got := first.Fields[otel.FieldTraceID]; got != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("trace_id = %v", got)
	}
	if !first.Timestamp.Equal(ts) {
		t.Fatalf("timestamp = %v, want %v", first.Timestamp, ts)
	}

	res, ok := first.Meta[otel.MetaResourceKey].(util.MapStr)
	if !ok || res[otel.ResourceHostName] != "web-1" {
		t.Fatalf("resource = %v", first.Meta[otel.MetaResourceKey])
	}

	// severity number derived from text when absent
	rec := otel.FromEvent(first)
	if rec.SeverityText != "ERROR" {
		t.Fatalf("severity text = %v", rec.SeverityText)
	}
}
