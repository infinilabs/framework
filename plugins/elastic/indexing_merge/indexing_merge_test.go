/* ©INFINI, All Rights Reserved. */

package indexing_merge

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

	out := normalizeDataStreamDoc([]byte(envelope))
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

	// @timestamp: payload 无显式 timestamp 时 observed_timestamp 优先于信封级;
	// timestamp 镜像之。
	if doc["@timestamp"] != "2026-08-23T14:38:32.982744Z" || doc["timestamp"] != "2026-08-23T14:38:32.982744Z" {
		t.Fatalf("timestamp fields: @=%v ts=%v", doc["@timestamp"], doc["timestamp"])
	}

	// metadata promotion.
	file, ok := doc["file"].(map[string]interface{})
	if !ok || file["path"] != "/var/log/system.log" {
		t.Fatalf("file metadata not promoted: %v", doc["file"])
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
		out := normalizeDataStreamDoc([]byte(env))
		var doc util.MapStr
		_ = json.Unmarshal(out, &doc)
		if doc["@timestamp"] != tc.want {
			t.Fatalf("%s: @timestamp = %v, want %v", tc.name, doc["@timestamp"], tc.want)
		}
	}
}

// TestNormalizeDataStreamDocBareDoc: non-envelope docs keep their fields
// and only get the @timestamp stamp.
func TestNormalizeDataStreamDocBareDoc(t *testing.T) {
	out := normalizeDataStreamDoc([]byte(`{"message":"raw","level":"info"}`))
	var doc util.MapStr
	_ = json.Unmarshal(out, &doc)
	if doc["message"] != "raw" || doc["level"] != "info" {
		t.Fatalf("bare doc fields lost: %v", doc)
	}
	if doc["timestamp"] == "" || doc["@timestamp"] == "" {
		t.Fatalf("bare doc missing timestamp stamps: %v", doc)
	}
}

// TestFingerprintID: deterministic id from dot-path fields — same values
// hash identically, different content diverges; when no key field resolves
// the raw message bytes are hashed instead so distinct docs stay distinct.
func TestFingerprintID(t *testing.T) {
	newProc := func(fields ...string) *IndexingMergeProcessor {
		p := &IndexingMergeProcessor{}
		p.config.KeyFields = fields
		return p
	}

	envelope := func(offset string) []byte {
		return []byte(`{
			"metadata":{"file":{"path":"/data/tmdb.csv","offset":` + offset + `}},
			"timestamp":"2026-08-27T04:20:21.349133Z",
			"payload":{"message":"row content"}
		}`)
	}

	proc := newProc("metadata.file.path", "metadata.file.offset")

	id1 := proc.fingerprintID(envelope("2028"))
	if len(id1) == 0 {
		t.Fatal("empty fingerprint")
	}
	if again := proc.fingerprintID(envelope("2028")); again != id1 {
		t.Fatalf("not deterministic: %v vs %v", id1, again)
	}
	if id2 := proc.fingerprintID(envelope("5698")); id2 == id1 {
		t.Fatal("different offset should yield different id")
	}

	// same file/offset but drifted ingest stamp must NOT change the id —
	// retransmitted events get re-stamped, that's what dedup has to survive
	drifted := []byte(`{
		"metadata":{"file":{"path":"/data/tmdb.csv","offset":2028}},
		"timestamp":"2026-08-27T04:20:31.000000Z",
		"payload":{"message":"row content"}
	}`)
	if id3 := proc.fingerprintID(drifted); id3 != id1 {
		t.Fatalf("id must be stable across timestamp drift: %v vs %v", id1, id3)
	}

	// no key fields present → raw-content fallback
	fallback := newProc("metadata.file.path")
	msgA := []byte(`{"payload":{"message":"unique-a"}}`)
	msgB := []byte(`{"payload":{"message":"unique-b"}}`)
	idA := fallback.fingerprintID(msgA)
	idB := fallback.fingerprintID(msgB)
	if idA == idB {
		t.Fatal("fallback must keep distinct messages apart")
	}
	if idAA := fallback.fingerprintID(msgA); idAA != idA {
		t.Fatal("fallback must stay deterministic")
	}

	// undecodable message → raw hash, no panic
	if junkID := proc.fingerprintID([]byte{0xff, 0xfe, 0x01}); len(junkID) == 0 {
		t.Fatal("undecodable input should still produce an id")
	}
}
