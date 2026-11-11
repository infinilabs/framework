package queue

import (
	"os"
	"testing"
	"time"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
)

// Regression tests for consumers whose persisted offset points at segment
// files that no longer exist (queue wiped / deleted and recreated, or
// retention removed files under a lagging consumer). AcquireConsumer used
// to return "not found, next segment greater than current write segment"
// and wedge pipeline consumers in an acquire-retry panic loop; the
// consumer must instead self-heal onto whatever data is still on disk.

func newSelfhealTestQueue(t *testing.T, name string) (*DiskBasedQueue, string) {
	t.Helper()
	global.Env().SystemConfig.PathConfig.Data = t.TempDir()
	dataPath := GetDataPath(name)
	_ = os.MkdirAll(dataPath, 0755)

	cfg := &DiskQueueConfig{
		MinMsgSize:          1,
		MaxMsgSize:          104857600,
		MaxBytesPerFile:     1024, // small segments: multiple files quickly
		SyncEveryRecords:    1,
		SyncTimeoutInMS:     10,
		WriteTimeoutInMS:    1000,
		EOFRetryDelayInMs:   100,
		ReadChanBuffer:      10,
		WriteChanBuffer:     10,
		AutoSkipCorruptFile: true,
	}
	return NewDiskQueueByConfig(name, dataPath, cfg), dataPath
}

func putTestRecords(t *testing.T, d *DiskBasedQueue, n int, payload string) {
	t.Helper()
	for i := 0; i < n; i++ {
		res := d.Put([]byte(payload))
		if res.Error != nil {
			t.Fatalf("put: %v", res.Error)
		}
	}
	time.Sleep(500 * time.Millisecond)
}

// Offset lands inside the range of segments, but every file up to (not
// including) the current write segment was wiped: the skip-forward walk
// must include the write segment itself, which it previously never
// checked.
func TestAcquireConsumerSkipsToWriteSegment(t *testing.T) {
	name := "test-selfheal-skip"
	d, _ := newSelfhealTestQueue(t, name)
	defer d.Close()

	putTestRecords(t, d, 200, "0123456789abcdef0123456789abcdef0123456789abcdef")
	if d.writeSegmentNum < 2 {
		t.Fatalf("expected several segments, got write segment %v", d.writeSegmentNum)
	}

	// wipe every segment file below the last written one (old data gone,
	// fresh data still sits in the newest segment file)
	for seg := int64(0); seg < d.writeSegmentNum-1; seg++ {
		if err := os.Remove(GetFileName(name, seg)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove segment %v: %v", seg, err)
		}
	}

	c, err := d.AcquireConsumer(&queue.QueueConfig{ID: name, Name: name}, &queue.ConsumerConfig{Name: "selfheal-skip"}, queue.NewOffset(1, 512))
	if err != nil {
		t.Fatalf("acquire with wiped offset should self-heal, got: %v", err)
	}
	defer c.Close()

	msgs, _, err := c.FetchMessages(&queue.Context{}, 500)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected the records retained in the write segment to be consumable")
	}
	t.Logf("self-healed onto write segment %v, fetched %v messages", d.writeSegmentNum, len(msgs))
}

// Offset is beyond the write segment entirely (queue deleted and
// recreated, writer restarted from a low segment): the consumer must
// re-anchor onto the earliest retained segment instead of erroring.
func TestAcquireConsumerResetsToEarliestRetainedData(t *testing.T) {
	name := "test-selfheal-reset"
	d, dataPath := newSelfhealTestQueue(t, name)

	putTestRecords(t, d, 200, "0123456789abcdef0123456789abcdef0123456789abcdef")
	d.Close()

	// wipe the whole queue dir except meta, then reopen the writer and
	// produce fresh data into low segments — the "queue recreated" state
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "meta.dat" {
			if err := os.Remove(dataPath + string(os.PathSeparator) + e.Name()); err != nil {
				t.Fatalf("remove %v: %v", e.Name(), err)
			}
		}
	}

	d2 := NewDiskQueueByConfig(name, dataPath, &DiskQueueConfig{
		MinMsgSize:          1,
		MaxMsgSize:          104857600,
		MaxBytesPerFile:     1048576,
		SyncEveryRecords:    1,
		SyncTimeoutInMS:     10,
		WriteTimeoutInMS:    1000,
		EOFRetryDelayInMs:   100,
		ReadChanBuffer:      10,
		WriteChanBuffer:     10,
		AutoSkipCorruptFile: true,
	})
	defer d2.Close()
	putTestRecords(t, d2, 5, "post-wipe message")

	// stale offset far beyond the recreated queue's write segment
	c, err := d2.AcquireConsumer(&queue.QueueConfig{ID: name, Name: name}, &queue.ConsumerConfig{Name: "selfheal-reset"}, queue.NewOffset(7, 104857842))
	if err != nil {
		t.Fatalf("acquire with offset beyond retained data should self-heal, got: %v", err)
	}
	defer c.Close()

	// the writer flushes asynchronously: poll until the 5 post-wipe
	// messages are all consumed
	total := 0
	deadline := time.Now().Add(5 * time.Second)
	for total < 5 && time.Now().Before(deadline) {
		msgs, _, err := c.FetchMessages(&queue.Context{}, 10)
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		total += len(msgs)
		time.Sleep(100 * time.Millisecond)
	}
	if total != 5 {
		t.Fatalf("expected the 5 post-wipe messages, got %v", total)
	}
}

// No data file survives at all (fully wiped queue, nothing written since):
// acquiring with a stale offset must succeed with the consumer parked at
// the write head — FetchMessages reports empty instead of erroring — and
// consumption must resume in place once the writer produces data.
func TestAcquireConsumerParksAtWriteHeadWhenNoData(t *testing.T) {
	name := "test-selfheal-park"
	d, dataPath := newSelfhealTestQueue(t, name)

	putTestRecords(t, d, 200, "0123456789abcdef0123456789abcdef0123456789abcdef")
	d.Close()

	// wipe the whole queue dir except meta and reopen the writer with no
	// data on disk
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "meta.dat" {
			if err := os.Remove(dataPath + string(os.PathSeparator) + e.Name()); err != nil {
				t.Fatalf("remove %v: %v", e.Name(), err)
			}
		}
	}

	d2 := NewDiskQueueByConfig(name, dataPath, &DiskQueueConfig{
		MinMsgSize:          1,
		MaxMsgSize:          104857600,
		MaxBytesPerFile:     1048576,
		SyncEveryRecords:    1,
		SyncTimeoutInMS:     10,
		WriteTimeoutInMS:    1000,
		EOFRetryDelayInMs:   100,
		ReadChanBuffer:      10,
		WriteChanBuffer:     10,
		AutoSkipCorruptFile: true,
	})
	defer d2.Close()
	time.Sleep(500 * time.Millisecond)

	// stale offset beyond everything: parks at the write head
	c, err := d2.AcquireConsumer(&queue.QueueConfig{ID: name, Name: name}, &queue.ConsumerConfig{Name: "selfheal-park", EOFRetryDelayInMs: 50}, queue.NewOffset(20, 5))
	if err != nil {
		t.Fatalf("acquire with no retained data should park, got: %v", err)
	}
	defer c.Close()

	msgs, _, err := c.FetchMessages(&queue.Context{}, 10)
	if err != nil {
		t.Fatalf("fetch on parked consumer should be a clean empty, got: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty fetch on parked consumer, got %v", len(msgs))
	}

	// data arrives: the parked consumer picks it up without re-acquire
	putTestRecords(t, d2, 3, "post-park message")
	total := 0
	deadline := time.Now().Add(5 * time.Second)
	for total < 3 && time.Now().Before(deadline) {
		msgs, _, err = c.FetchMessages(&queue.Context{}, 10)
		if err != nil {
			t.Fatalf("fetch after data arrived: %v", err)
		}
		total += len(msgs)
		time.Sleep(100 * time.Millisecond)
	}
	if total != 3 {
		t.Fatalf("expected 3 post-park messages, got %v", total)
	}
}
