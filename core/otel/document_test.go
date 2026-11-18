// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software. For commercial licensing, contact us at:
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

package otel

import (
	"encoding/json"
	"testing"

	"infini.sh/framework/core/util"
)

// TestNormalizeDataStreamDoc verifies the otel-envelope → data stream doc
// normalization: payload promotion, @timestamp derivation priority, and
// metadata promotion (file / log_type / log_kind).
func TestNormalizeDataStreamDoc(t *testing.T) {
	envelope := `{"metadata":{"file":{"path":"/var/log/system.log","offset":123},"log_type":"text","log_kind":"server"},
		"payload":{"message":"adding data stream [system-logs]","observed_timestamp":"2026-08-23T14:38:32.982744Z"},
		"timestamp":"2026-08-23T14:38:32Z"}`

	out := NormalizeDataStreamDoc([]byte(envelope))
	var doc util.MapStr
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal normalized doc: %v", err)
	}

	// payload promoted to top level; envelope keys dropped.
	if doc["message"] != "adding data stream [system-logs]" {
		t.Fatalf("message not promoted: %v", doc)
	}
	if _, ok := doc["payload"]; ok {
		t.Fatalf("payload key should not survive: %v", doc)
	}

	// @timestamp: when the payload carries no explicit timestamp,
	// observed_timestamp wins over the envelope-level one. @timestamp is
	// the only real timestamp — the legacy "timestamp" mirror is no longer
	// written.
	if doc["@timestamp"] != "2026-08-23T14:38:32.982744Z" {
		t.Fatalf("@timestamp = %v", doc["@timestamp"])
	}
	if _, ok := doc["timestamp"]; ok {
		t.Fatalf("timestamp mirror must not be written: %v", doc["timestamp"])
	}
	// observed_timestamp folds into @timestamp and is stripped at output:
	// zero readers, and the chain records the semantics per hop
	// (@timestamp / @agent.observed_at / @gateway.observed_at).
	if _, ok := doc["observed_timestamp"]; ok {
		t.Fatalf("observed_timestamp must not survive at output: %v", doc["observed_timestamp"])
	}

	// metadata promotion: file origin no longer promotes to a top-level
	// "file" (collection provenance lives on @agent.file_*); only the
	// log_type/log_kind labels promote.
	if _, ok := doc["file"]; ok {
		t.Fatalf("top-level file promotion is gone: %v", doc["file"])
	}
	if doc["log_type"] != "text" || doc["log_kind"] != "server" {
		t.Fatalf("label fields not promoted: %v", doc)
	}
}

// TestNormalizeDataStreamDocTimestampPriority: payload.timestamp wins over
// observed_timestamp and the envelope timestamp.
func TestNormalizeDataStreamDocTimestampPriority(t *testing.T) {
	for _, tc := range []struct {
		name, payload, envelope, want string
	}{
		{"payload timestamp first", `"timestamp":"2026-01-01T00:00:00Z","observed_timestamp":"2026-02-02T00:00:00Z"`, "2026-03-03T00:00:00Z", "2026-01-01T00:00:00Z"},
		{"observed over envelope", `"observed_timestamp":"2026-02-02T00:00:00Z"`, "2026-03-03T00:00:00Z", "2026-02-02T00:00:00Z"},
		{"envelope fallback", `"unused":"x"`, "2026-03-03T00:00:00Z", "2026-03-03T00:00:00Z"},
	} {
		env := `{"payload":{` + tc.payload + `},"timestamp":"` + tc.envelope + `"}`
		out := NormalizeDataStreamDoc([]byte(env))
		var doc util.MapStr
		_ = json.Unmarshal(out, &doc)
		if doc["@timestamp"] != tc.want {
			t.Fatalf("%s: @timestamp = %v, want %v", tc.name, doc["@timestamp"], tc.want)
		}
		if _, ok := doc["timestamp"]; ok {
			t.Fatalf("%s: payload/envelope timestamp must fold into @timestamp only: %v", tc.name, doc)
		}
	}
}

// TestNormalizeDataStreamDocBareDoc: non-envelope docs keep their fields
// and only get the @timestamp stamp (single truth — no mirror).
func TestNormalizeDataStreamDocBareDoc(t *testing.T) {
	out := NormalizeDataStreamDoc([]byte(`{"message":"raw","level":"info"}`))
	var doc util.MapStr
	_ = json.Unmarshal(out, &doc)
	if doc["message"] != "raw" || doc["level"] != "info" {
		t.Fatalf("bare doc fields lost: %v", doc)
	}
	if doc["@timestamp"] == "" {
		t.Fatalf("bare doc missing @timestamp: %v", doc)
	}
	if _, ok := doc["timestamp"]; ok {
		t.Fatalf("bare doc must not gain a timestamp mirror: %v", doc)
	}

	// bare doc already carrying @timestamp keeps it (no re-stamp)
	keep := NormalizeDataStreamDoc([]byte(`{"message":"kept","@timestamp":"2026-05-05T00:00:00Z"}`))
	var kd util.MapStr
	_ = json.Unmarshal(keep, &kd)
	if kd["@timestamp"] != "2026-05-05T00:00:00Z" {
		t.Fatalf("bare doc @timestamp must be kept: %v", kd["@timestamp"])
	}
}

// A bare document carrying only the OTel wire field must fold
// observed_timestamp into @timestamp instead of losing it to "now" — it is
// the document's best-known time when no @timestamp/timestamp exists.
func TestNormalizeDataStreamDoc_BareDocObservedTimestamp(t *testing.T) {
	out := NormalizeDataStreamDoc([]byte(`{"message":"raw","observed_timestamp":"2026-10-15T00:00:00.123Z"}`))
	doc := util.MapStr{}
	if err := util.FromJSONBytes(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["@timestamp"] != "2026-10-15T00:00:00.123Z" {
		t.Fatalf("@timestamp must come from the bare doc's observed_timestamp: %v", doc["@timestamp"])
	}
	if _, ok := doc["observed_timestamp"]; ok {
		t.Fatal("observed_timestamp must not survive at output")
	}
}
