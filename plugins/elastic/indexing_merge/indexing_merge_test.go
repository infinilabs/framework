/* ©INFINI, All Rights Reserved. */

package indexing_merge

import (
	"testing"
)

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
