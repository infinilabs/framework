package queue

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
)

type memKV struct{ data map[string][]byte }

func (m *memKV) Open() error                                     { return nil }
func (m *memKV) Close() error                                    { return nil }
func (m *memKV) GetValue(b string, k []byte) ([]byte, error) {
	if v, ok := m.data[string(k)]; ok { return append([]byte(nil), v...), nil }
	return nil, nil
}
func (m *memKV) GetCompressedValue(b string, k []byte) ([]byte, error) { return m.GetValue(b, k) }
func (m *memKV) AddValueCompress(b string, k, v []byte) error          { return m.AddValue(b, k, v) }
func (m *memKV) AddValue(b string, k, v []byte) error {
	m.data[string(k)] = append([]byte(nil), v...)
	return nil
}
func (m *memKV) ExistsKey(b string, k []byte) (bool, error) {
	_, ok := m.data[string(k)]
	return ok, nil
}
func (m *memKV) DeleteKey(b string, k []byte) error {
	delete(m.data, string(k))
	return nil
}

func init() {
	kv.Register("test-mem-kv", &memKV{data: map[string][]byte{}})
}

func TestEmptyRemovesConsumedSegments(t *testing.T) {
	global.Env().SystemConfig.PathConfig.Data = t.TempDir()
	name := "test-empty-queue"
	dataPath := GetDataPath(name)
	_ = os.MkdirAll(dataPath, 0755)

	cfg := &DiskQueueConfig{
		MinMsgSize:       1,
		MaxMsgSize:       104857600,
		MaxBytesPerFile:  1024, // small segments: multiple files quickly
		SyncEveryRecords: 1,
		SyncTimeoutInMS:  10,
		WriteTimeoutInMS: 1000,
		EOFRetryDelayInMs: 100,
		ReadChanBuffer:   10,
		WriteChanBuffer:  10,
	}
	d := NewDiskQueueByConfig(name, dataPath, cfg)
	defer d.Close()

	// write enough records to roll several 1KB segments
	for i := 0; i < 200; i++ {
		res := d.Put([]byte("0123456789abcdef0123456789abcdef0123456789abcdef"))
		if res.Error != nil {
			t.Fatalf("put: %v", res.Error)
		}
	}
	time.Sleep(500 * time.Millisecond)

	// consume everything so depth returns to 0 (consumed segments retained)
	read := 0
	for read < 200 {
		select {
		case <-d.ReadChan():
			read++
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout consuming, read=%v", read)
		}
	}
	time.Sleep(500 * time.Millisecond)

	filesBefore := 0
	filepath.Walk(dataPath, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			filesBefore++
		}
		return nil
	})
	if filesBefore == 0 {
		t.Fatal("no segment files before empty")
	}

	// the behavior under test: Empty drops ALL segment files, fast
	start := time.Now()
	if err := d.Empty(); err != nil {
		t.Fatalf("empty: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("empty took too long: %v", elapsed)
	}

	time.Sleep(500 * time.Millisecond) // async unlink
	filesAfter := 0
	filepath.Walk(dataPath, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && filepath.Base(p) != "meta.dat" {
			filesAfter++
		}
		return nil
	})
	if filesAfter != 0 {
		t.Fatalf("expected 0 segment files after empty, got %v", filesAfter)
	}
	if d.Depth() != 0 {
		t.Fatalf("expected depth 0, got %v", d.Depth())
	}
}
