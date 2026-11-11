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

// The agent identity must survive the OTLP LogRecord projection: FromEvent
// merges it into Resource attributes, ToEvent restores it when the inbound
// event has none. A resource key set upstream must not be overwritten, and
// an event that already carries its Agent must keep it as-is.
func TestAgentIdentitySurvivesLogRecord(t *testing.T) {
	e := &event.Event{
		Timestamp: time.Now().UTC(),
		Agent: &event.AgentMeta{
			AgentID:  "agent-7",
			Hostname: "web-1",
			MajorIP:  "10.0.0.5",
			Tags:     []string{"prod", "edge"},
		},
		Fields: util.MapStr{FieldMessage: "m"},
	}

	rec := FromEvent(e)
	if rec.Resource[ResourceAgentID] != "agent-7" || rec.Resource[ResourceHostName] != "web-1" {
		t.Fatalf("agent identity not merged into resource: %v", rec.Resource)
	}
	if rec.Resource[ResourceHostIP] != "10.0.0.5" {
		t.Fatalf("host_ip = %v", rec.Resource)
	}

	back := rec.ToEvent()
	if back.Agent == nil || back.Agent.AgentID != "agent-7" || back.Agent.Hostname != "web-1" ||
		back.Agent.MajorIP != "10.0.0.5" {
		t.Fatalf("agent not restored: %v", back.Agent)
	}

	// upstream resource keys win over the agent merge
	e2 := &event.Event{
		Agent:  &event.AgentMeta{AgentID: "agent-7", Hostname: "web-1"},
		Meta:   util.MapStr{MetaResourceKey: util.MapStr{ResourceHostName: "already-set"}},
		Fields: util.MapStr{FieldMessage: "m"},
	}
	rec2 := FromEvent(e2)
	if rec2.Resource[ResourceHostName] != "already-set" {
		t.Fatalf("existing resource keys must not be overwritten: %v", rec2.Resource)
	}

	// no agent anywhere: nothing restored, no empty AgentMeta injected
	e3 := &event.Event{Fields: util.MapStr{FieldMessage: "m"}}
	back3 := FromEvent(e3).ToEvent()
	if back3.Agent != nil {
		t.Fatalf("agent must stay nil without identity, got %v", back3.Agent)
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

// The agent's flat metadata must survive a decode→encode pass (what
// for_each does to every record) byte-for-byte: no nested resource key,
// no duplication.
func TestEnvelopeAgentMetadataRoundTripStable(t *testing.T) {
	body := []byte(`{"metadata":{"log_type":"text"},"payload":{"message":"hello"},"timestamp":"2026-08-18T10:00:00Z"}`)

	e, err := DecodeEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := e.Meta[MetaResourceKey].(util.MapStr)
	if !ok || res["log_type"] != "text" {
		t.Fatalf("resource = %v", e.Meta)
	}
	if _, ok := e.Meta["log_type"]; ok {
		t.Fatalf("metadata must not be duplicated at Meta top level: %v", e.Meta)
	}

	out, err := EncodeEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("round trip changed the wire shape:\n in: %s\nout: %s", body, out)
	}
}

// A metadata map that already carries an explicit resource collection
// (EncodeEnvelope of a richer event.Meta) round-trips without nesting.
func TestEnvelopeStructuredMetadataRoundTrip(t *testing.T) {
	e := &event.Event{
		Timestamp: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
		Fields:    util.MapStr{FieldMessage: "x"},
		Meta: util.MapStr{
			MetaResourceKey: util.MapStr{ResourceHostName: "db-1"},
			"custom":        "y",
		},
	}
	data, err := EncodeEnvelope(e)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := back.Meta[MetaResourceKey].(util.MapStr)
	if !ok || res[ResourceHostName] != "db-1" {
		t.Fatalf("resource = %v", back.Meta)
	}
	if _, nested := res[MetaResourceKey]; nested {
		t.Fatalf("double-nested resource: %v", back.Meta)
	}
	if back.Meta["custom"] != "y" {
		t.Fatalf("custom meta key lost: %v", back.Meta)
	}
}

// A bare JSON map payload is treated as the record's Attributes.
func TestDecodeEnvelopeBareMapFallback(t *testing.T) {
	e, err := DecodeEnvelope([]byte(`{"foo":"bar","n":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.Fields["foo"] != "bar" {
		t.Fatalf("bare map must land in Fields, got %v", e.Fields)
	}
	if len(e.Meta) != 0 {
		t.Fatalf("bare map must not synthesize metadata: %v", e.Meta)
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
