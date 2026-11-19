package kafka_queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/queue"
)

func newTestConsumer() *Consumer {
	return &Consumer{
		qCfg:      &queue.QueueConfig{ID: "test-topic", Name: "test-topic"},
		partNext:  map[int32]int64{},
		fetchTail: queue.NewOffset(-1, 0), // no fetch yet: nothing drains
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

		assert.False(t, drained, "no fetch tail recorded: every commit is incremental")
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
	// batch mixed p1 (3 records) and p3 (2 records); the batch's last
	// record was p3's, so the fetch tail is (3,2) and the commit says p3
	c.partNext[1] = 3
	c.partNext[3] = 2
	c.fetchTail = queue.NewOffset(3, 2)

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
	assert.False(t, drained, "no fetch tail recorded: incremental commit")
	inner, ok := m["test-topic"]
	assert.True(t, ok)
	assert.Len(t, inner, 1)
	assert.Equal(t, int64(7), inner[2].Offset)

	for p := range inner {
		assert.Equal(t, int32(2), p, "unexpected partition %d committed", p)
	}
}

// When the committed position is not the fetch tail — bulk-indexing buffer
// boundaries, interrupted batches, admin rewinds — only the requested
// position may be committed: the other partitions' records may still be in
// flight. A position trailing the partition's tracked progress rewinds the
// tracking so a later flush cannot skip the gap.
func TestBuildCommitRequest_TrailingOffsetCommitsExplicitOnly(t *testing.T) {
	c := newTestConsumer()
	c.partNext[3] = 5 // fetched p3:0-4, ack says p3@2
	c.partNext[1] = 3
	c.fetchTail = queue.NewOffset(3, 5)

	m, drained := c.buildCommitRequest(queue.NewOffset(3, 2))
	assert.False(t, drained)
	inner, ok := m["test-topic"]
	assert.True(t, ok)
	assert.Len(t, inner, 1, "only the explicit partition must be committed")
	assert.Equal(t, int64(2), inner[3].Offset)
	assert.Equal(t, int64(2), c.partNext[3], "tracking must rewind to the committed position")
	_, committed := inner[1]
	assert.False(t, committed, "p1's unprocessed progress must not be committed")

	// at the fetch tail the whole buffer was processed: flush everything
	m2, drained2 := c.buildCommitRequest(queue.NewOffset(3, 5))
	assert.True(t, drained2)
	assert.Len(t, m2["test-topic"], 2, "the tail commit flushes the whole map")
	assert.Equal(t, int64(5), m2["test-topic"][3].Offset, "explicit tail position wins over the rewound tracking")
	assert.Equal(t, int64(3), m2["test-topic"][1].Offset)
}

// Regression for the bulk-indexing incremental commit pattern: offset
// advances per popped message and commits happen at buffer-flush
// boundaries. A chunk boundary that exhausts one partition's fetched
// records (commit == that partition's tracked progress) must NOT flush the
// other partitions — their records are still in flight and committing them
// here would permanently skip them on crash. Only the fetch tail may flush.
func TestBuildCommitRequest_IncrementalCommitKeepsOtherPartitions(t *testing.T) {
	c := newTestConsumer()
	// fetch returned p1:0-9 and p3:0-9; the batch's last record was p3's
	c.partNext[1] = 10
	c.partNext[3] = 10
	c.fetchTail = queue.NewOffset(3, 10)

	// chunk 1 flushed p1:0-4: explicit-only, no rewind of p3, no flush
	m1, drained1 := c.buildCommitRequest(queue.NewOffset(1, 5))
	assert.False(t, drained1)
	assert.Len(t, m1["test-topic"], 1)
	assert.Equal(t, int64(5), m1["test-topic"][1].Offset)
	_, flushed := m1["test-topic"][3]
	assert.False(t, flushed, "in-flight p3 must not be committed by p1's chunk commit")

	// chunk 2 flushed p1:5-9 — equals p1's tracked progress, but is NOT the
	// fetch tail: the old "caught-up partitions drain the buffer" heuristic
	// flushed p3 here and permanently skipped its unsubmitted records
	m2, drained2 := c.buildCommitRequest(queue.NewOffset(1, 10))
	assert.False(t, drained2)
	assert.Len(t, m2["test-topic"], 1, "caught-up partition must still not flush the buffer")
	_, flushed = m2["test-topic"][3]
	assert.False(t, flushed)
	// CommitOffset folds a successful commit's position back into the
	// tracking (chunk 2 reached the broker)
	c.partNext[1] = 10

	// chunk 3 flushes the rest of the fetch (p3:0-9) — the fetch tail:
	// every fetched record is submitted, the whole map commits
	m3, drained3 := c.buildCommitRequest(queue.NewOffset(3, 10))
	assert.True(t, drained3)
	assert.Len(t, m3["test-topic"], 2)
	assert.Equal(t, int64(10), m3["test-topic"][1].Offset)
	assert.Equal(t, int64(10), m3["test-topic"][3].Offset)
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
