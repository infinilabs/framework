package otel

import (
	"testing"
	"time"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

func benchEvent() *event.Event {
	return &event.Event{
		Timestamp: time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC),
		Fields: util.MapStr{"message": "ERROR payment-svc: connection refused to db-1:5432, retry 3", "log_level": "ERROR", "service_name": "payment-svc"},
	}
}

func BenchmarkEnvelopeRoundTrip(b *testing.B) {
	data, _ := EncodeEnvelope(benchEvent())
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec, _ := DecodeEnvelope(data)
		_, _ = EncodeEnvelope(rec)
	}
}
