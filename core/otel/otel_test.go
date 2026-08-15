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

package otel

import (
	"testing"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

func TestEventRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	e := &event.Event{
		Timestamp: ts,
		Fields: util.MapStr{
			FieldLogLevel: "WARN",
			FieldMessage:  "disk usage 91%",
			"device":      "sda1",
		},
		Meta: util.MapStr{
			MetaResourceKey: util.MapStr{ResourceHostName: "db-1"},
		},
	}

	rec := FromEvent(e)
	if rec.SeverityText != "WARN" {
		t.Fatalf("severity = %v", rec.SeverityText)
	}
	if rec.Body != "disk usage 91%" {
		t.Fatalf("body = %v", rec.Body)
	}
	if rec.Resource[ResourceHostName] != "db-1" {
		t.Fatalf("resource = %v", rec.Resource)
	}

	back := rec.ToEvent()
	if !back.Timestamp.Equal(ts) {
		t.Fatalf("timestamp = %v", back.Timestamp)
	}
	if back.Fields[FieldMessage] != "disk usage 91%" || back.Fields["device"] != "sda1" {
		t.Fatalf("fields = %v", back.Fields)
	}
	res, ok := back.Meta[MetaResourceKey].(util.MapStr)
	if !ok || res[ResourceHostName] != "db-1" {
		t.Fatalf("resource = %v", back.Meta[MetaResourceKey])
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	e := &event.Event{
		Timestamp: ts,
		Agent:     &event.AgentMeta{AgentID: "agent-1"},
		Fields:    util.MapStr{FieldMessage: "hello", FieldLogLevel: "INFO"},
		Meta:      util.MapStr{MetaResourceKey: util.MapStr{ResourceServiceName: "svc"}},
	}

	data, err := EncodeEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Fields[FieldMessage] != "hello" {
		t.Fatalf("message = %v", rec.Fields[FieldMessage])
	}
	if rec.Agent == nil || rec.Agent.AgentID != "agent-1" {
		t.Fatalf("agent = %v", rec.Agent)
	}
	if !rec.Timestamp.Equal(ts) {
		t.Fatalf("timestamp = %v, want %v", rec.Timestamp, ts)
	}
	res, ok := rec.Meta[MetaResourceKey].(util.MapStr)
	if !ok || res[ResourceServiceName] != "svc" {
		t.Fatalf("resource = %v", rec.Meta[MetaResourceKey])
	}
}

func TestSeverityNumberFromText(t *testing.T) {
	cases := map[string]int32{
		"TRACE": SeverityTrace1,
		"debug": SeverityDebug,
		"INFO":  SeverityInfo,
		"WARN":  SeverityWarn,
		"warn":  SeverityWarn,
		"error": SeverityError,
		"FATAL": SeverityFatal,
		"":      0,
	}
	for text, want := range cases {
		if got := SeverityNumberFromText(text); got != want {
			t.Fatalf("SeverityNumberFromText(%q) = %d, want %d", text, got, want)
		}
	}
}
