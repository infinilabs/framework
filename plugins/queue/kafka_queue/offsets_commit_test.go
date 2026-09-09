package kafka_queue

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"infini.sh/framework/core/queue"
)

// CommitOffset historically committed every offset to partition 0
// (hard-wired map index), corrupting multi-partition topics: each
// partition's progress overwrote partition 0's commit and its own position
// was never persisted. The partition must come from the offset's segment —
// exactly what FetchMessages stores there (NewOffset(r.Partition, r.Offset)).
func TestOffsetsToCommit_UsesOffsetPartition(t *testing.T) {
	qCfg := &queue.QueueConfig{ID: "test-topic", Name: "test-topic"}

	for partition := int64(0); partition < 5; partition++ {
		off := queue.NewOffset(partition, 100+partition)
		m := offsetsToCommit(qCfg, off)

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
