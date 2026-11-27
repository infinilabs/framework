/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// A JSON object becomes the record's fields wholesale; the codec hint is
// stamped in metadata (not fields, so the encoded payload stays clean) and
// the document's @timestamp hydrates Event.Timestamp.
func TestDocumentCodec_Decode(t *testing.T) {
	rec, err := documentCodec{}.Decode([]byte(`{"@timestamp":"2026-01-02T03:04:05Z","host":"a","message":"hi"}`))
	assert.NoError(t, err)
	assert.Equal(t, "a", rec.Fields["host"])
	assert.Equal(t, "hi", rec.Fields["message"])
	assert.Equal(t, "document", CodecHint(rec), "queue outputs must re-encode with the document codec")

	want, _ := time.Parse(time.RFC3339, "2026-01-02T03:04:05Z")
	assert.Equal(t, want, rec.Timestamp, "event time must hydrate from the document")

	// an empty JSON object is still a document, not a text line
	recEmpty, err := documentCodec{}.Decode([]byte(`{}`))
	assert.NoError(t, err)
	assert.NotContains(t, recEmpty.Fields, "message", "empty JSON object must not become a message")
	assert.Equal(t, "document", CodecHint(recEmpty))

	// non-object payloads land as one text line; hint and timestamp defaults
	rec2, err := documentCodec{}.Decode([]byte("plain line"))
	assert.NoError(t, err)
	assert.Equal(t, "plain line", rec2.Fields["message"])
	assert.Equal(t, "document", CodecHint(rec2))
	assert.False(t, rec2.Timestamp.IsZero())
}

// Encoding mirrors Event.Timestamp back into @timestamp so edits through
// PutValue survive the queue round-trip; an untouched round-trip writes the
// value the document carried in.
func TestDocumentCodec_EncodeRoundTrip(t *testing.T) {
	orig, err := documentCodec{}.Decode([]byte(`{"@timestamp":"2026-01-02T03:04:05Z","k":"v"}`))
	assert.NoError(t, err)

	b, err := documentCodec{}.Encode(orig)
	assert.NoError(t, err)
	rec, err := documentCodec{}.Decode(b)
	assert.NoError(t, err)
	assert.Equal(t, "v", rec.Fields["k"])
	assert.Equal(t, orig.Timestamp, rec.Timestamp, "round-trip must preserve event time")

	// a processor time edit (PutValue writes Event.Timestamp only) must not
	// be lost on re-encode
	newTs := time.Date(2027, 3, 4, 5, 6, 7, 0, time.UTC)
	_, _ = orig.PutValue("@timestamp", newTs)
	b2, err := documentCodec{}.Encode(orig)
	assert.NoError(t, err)
	assert.Contains(t, string(b2), "2027-03-04T05:06:07", "edited timestamp must survive encoding")

	// zero timestamp is never written as a bogus date
	b3, err := documentCodec{}.Encode(&event.Event{Fields: util.MapStr{"k": "v"}})
	assert.NoError(t, err)
	assert.NotContains(t, string(b3), "@timestamp")
}
