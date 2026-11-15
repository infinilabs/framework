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
func TestBuildCommitRequest_UsesOffsetPartition(t *testing.T) {
	c := newTestConsumer()

	for partition := int64(0); partition < 5; partition++ {
		off := queue.NewOffset(partition, 100+partition)
		m, drained := c.buildCommitRequest(off)

		assert.True(t, drained, "no fetched progress: the explicit offset drains by definition")
		inner, ok := m["test-topic"]
		assert.True(t, ok, "topic map must exist")
		assert.Len(t, inner, 1, "exactly one partition must be committed")
		assert.Equal(t, int64(100+partition), inner[int32(partition)].Offset)
	}
}

// A batch fetched across several partitions carries one (partition, offset)
// pair at commit time — the last record's — while the other partitions'
// progress lives only in partNext. When the acknowledged position has
// caught up with its partition's tracked progress, the whole batch was
// processed and the full map must be flushed.
func TestBuildCommitRequest_FlushesAllPartitionsOfDrainedBatch(t *testing.T) {
	c := newTestConsumer()
	// batch mixed p1 (3 records) and p3 (2 records); commit says p3
	c.partNext[1] = 3
	c.partNext[3] = 2

	m, drained := c.buildCommitRequest(queue.NewOffset(3, 2))
	assert.True(t, drained)
	inner := m["test-topic"]
	assert.Len(t, inner, 2, "both batch partitions must be committed")

	assert.Equal(t, int64(3), inner[1].Offset, "p1 progress comes from partNext")
	assert.Equal(t, int64(2), inner[3].Offset, "p3 progress is the explicit offset")
}

// Zero-progress placeholders must never be committed: an explicit offset for
// partition 2 alone yields exactly one entry, not a stale (p2 → 0) write
// that would rewind the group to the log start.
func TestBuildCommitRequest_SkipsZeroProgress(t *testing.T) {
	c := newTestConsumer()

	m, drained := c.buildCommitRequest(queue.NewOffset(2, 7))
	assert.True(t, drained)
	inner, ok := m["test-topic"]
	assert.True(t, ok)
	assert.Len(t, inner, 1)
	assert.Equal(t, int64(7), inner[2].Offset)

	for p := range inner {
		assert.Equal(t, int32(2), p, "unexpected partition %d committed", p)
	}
}

// When the acknowledged position trails the fetched progress the batch was
// interrupted mid-processing (the framework keeps the previous offset for
// at-least-once redelivery) or the caller is an explicit rewind (admin
// reset). Only the requested position may be committed — flushing the map
// here would permanently skip records that were fetched but never
// processed, or silently ignore the reset. The partition's tracking must
// be rewound to the committed position.
func TestBuildCommitRequest_TrailingOffsetCommitsExplicitOnly(t *testing.T) {
	c := newTestConsumer()
	c.partNext[3] = 5 // fetched p3:0-4, ack says p3@2
	c.partNext[1] = 3

	m, drained := c.buildCommitRequest(queue.NewOffset(3, 2))
	assert.False(t, drained)
	inner, ok := m["test-topic"]
	assert.True(t, ok)
	assert.Len(t, inner, 1, "only the explicit partition must be committed")
	assert.Equal(t, int64(2), inner[3].Offset)
	assert.Equal(t, int64(2), c.partNext[3], "tracking must rewind to the committed position")
	_, committed := inner[1]
	assert.False(t, committed, "p1's unprocessed-progress must not be committed")

	// equal position drains (framework NextOffset == partNext after a fully
	// processed batch)
	m2, drained2 := c.buildCommitRequest(queue.NewOffset(3, 2))
	assert.True(t, drained2)
	assert.Len(t, m2["test-topic"], 2, "after the rewind, the follow-up commit flushes the map")
}

// Progress for partitions lost in a rebalance must be forgotten: a later
// commit flushing the stale entries would overwrite the new owner's
// committed positions with this consumer's old ones.
func TestDropPartitionProgress(t *testing.T) {
	c := newTestConsumer()
	c.partNext[1] = 3
	c.partNext[3] = 2

	c.dropPartitionProgress(map[string][]int32{"test-topic": {1}})
	assert.NotContains(t, c.partNext, int32(1))
	assert.Equal(t, int64(2), c.partNext[3], "owned partitions must be kept")

	c.dropPartitionProgress(map[string][]int32{"other-topic": {3}})
	assert.Equal(t, int64(2), c.partNext[3], "other topics must not touch this consumer")

	c.dropPartitionProgress(map[string][]int32{"test-topic": {3}})
	assert.Empty(t, c.partNext)
}
