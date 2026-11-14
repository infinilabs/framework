package kafka_queue

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"infini.sh/framework/core/queue"
)

func newTestConsumer() *Consumer {
	return &Consumer{
		qCfg:     &queue.QueueConfig{ID: "test-topic", Name: "test-topic"},
		partNext: map[int32]int64{},
	}
}

// CommitOffset historically committed every offset to partition 0
// (hard-wired map index), corrupting multi-partition topics: each
// partition's progress overwrote partition 0's commit and its own position
// was never persisted. The partition must come from the offset's segment —
// exactly what FetchMessages stores there (NewOffset(r.Partition, r.Offset)).
func TestOffsetsToCommit_UsesOffsetPartition(t *testing.T) {
	c := newTestConsumer()

	for partition := int64(0); partition < 5; partition++ {
		off := queue.NewOffset(partition, 100+partition)
		m := c.offsetsToCommit(off)

		inner, ok := m["test-topic"]
		assert.True(t, ok, "topic map must exist")
		assert.Len(t, inner, 1, "exactly one partition entry (partition %d)", partition)

		eo, ok := inner[int32(partition)]
		assert.True(t, ok, "entry must be keyed by the offset's own partition %d", partition)
		assert.Equal(t, int64(100+partition), eo.Offset)
		assert.Equal(t, int32(-1), eo.Epoch)

		// no other partition — in particular 0 — may receive this commit
		for p := range inner {
			assert.Equal(t, int32(partition), p, "commit leaked to partition %d", p)
		}
	}
}

// A batch fetched across several partitions carries one (partition, offset)
// pair at commit time — the last record's — while the other partitions'
// progress lives only in partNext. offsetsToCommit must flush the whole map
// alongside the explicit pair, otherwise those partitions are refetched and
// their records delivered twice.
func TestOffsetsToCommit_FlushesAllPartitionsOfBatch(t *testing.T) {
	c := newTestConsumer()
	// batch mixed p1 (3 records) and p3 (2 records); commit says p3
	c.partNext[1] = 3
	c.partNext[3] = 2

	m := c.offsetsToCommit(queue.NewOffset(3, 2))
	inner := m["test-topic"]
	assert.Len(t, inner, 2, "both batch partitions must be committed")

	assert.Equal(t, int64(3), inner[1].Offset, "p1 progress comes from partNext")
	assert.Equal(t, int64(2), inner[3].Offset, "p3 progress is the explicit offset")
}

// Zero-progress placeholders must never be committed: an explicit offset for
// partition 2 alone yields exactly one entry, not a stale (p2 → 0) write
// that would rewind the group to the log start.
func TestOffsetsToCommit_SkipsZeroProgress(t *testing.T) {
	c := newTestConsumer()

	m := c.offsetsToCommit(queue.NewOffset(2, 7))
	inner, ok := m["test-topic"]
	assert.True(t, ok)
	assert.Len(t, inner, 1)
	assert.Equal(t, int64(7), inner[2].Offset)

	for p := range inner {
		assert.Equal(t, int32(2), p, "unexpected partition %d committed", p)
	}
}
