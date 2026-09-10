package queue

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync/atomic"
	"testing"
	"time"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
)

type concKV struct{ data map[string][]byte }

func (m *concKV) Open() error  { return nil }
func (m *concKV) Close() error { return nil }
func (m *concKV) GetValue(b string, k []byte) ([]byte, error) {
	if v, ok := m.data[string(k)]; ok {
		return append([]byte(nil), v...), nil
	}
	return nil, nil
}
func (m *concKV) GetCompressedValue(b string, k []byte) ([]byte, error) { return m.GetValue(b, k) }
func (m *concKV) AddValueCompress(b string, k, v []byte) error         { return m.AddValue(b, k, v) }
func (m *concKV) AddValue(b string, k, v []byte) error {
	m.data[string(k)] = append([]byte(nil), v...)
	return nil
}
func (m *concKV) ExistsKey(b string, k []byte) (bool, error) {
	_, ok := m.data[string(k)]
	return ok, nil
}
func (m *concKV) DeleteKey(b string, k []byte) error {
	delete(m.data, string(k))
	return nil
}

func init() {
	kv.Register("test-conc-kv", &concKV{data: map[string][]byte{}})
}

func TestEmptyUnderConcurrentTraffic(t *testing.T) {
	global.Env().SystemConfig.PathConfig.Data = t.TempDir()
	name := "test-empty-conc"
	dataPath := GetDataPath(name)
	_ = os.MkdirAll(dataPath, 0755)

	cfg := &DiskQueueConfig{
		MinMsgSize: 1, MaxMsgSize: 104857600,
		MaxBytesPerFile: 1024, SyncEveryRecords: 1, SyncTimeoutInMS: 10,
		WriteTimeoutInMS: 1000, EOFRetryDelayInMs: 100,
		ReadChanBuffer: 10, WriteChanBuffer: 10,
		PrepareFilesToRead: true,
	}
	d := NewDiskQueueByConfig(name, dataPath, cfg)
	defer d.Close()

	var stop int32
	// producer
	go func() {
		for atomic.LoadInt32(&stop) == 0 {
			d.Put([]byte("0123456789abcdef0123456789abcdef0123456789abcdef"))
			time.Sleep(time.Millisecond)
		}
	}()
	// consumer
	go func() {
		for atomic.LoadInt32(&stop) == 0 {
			select {
			case <-d.ReadChan():
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()

	time.Sleep(2 * time.Second) // let segments accumulate

	// simulate orphaned segments from earlier eras ABOVE the in-memory counter
	// (metadata loss across restarts) — they must be purged too
	for _, orphan := range []string{"000000200.dat", "000000201.dat", "000000201.dat.zstd"} {
		if werr := os.WriteFile(filepath.Join(dataPath, orphan), []byte("orphan"), 0644); werr != nil {
			t.Fatal(werr)
		}
	}

	start := time.Now()
	err := d.Empty()
	elapsed := time.Since(start)
	atomic.StoreInt32(&stop, 1)
	if err != nil {
		// dump goroutines to see where ioLoop / deleteAllFiles is stuck
		_ = pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
		t.Fatalf("empty: %v (took %v)", err, elapsed)
	}
	t.Logf("empty took %v", elapsed)

	time.Sleep(1 * time.Second)
	files := 0
	filepath.Walk(dataPath, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			files++
		}
		return nil
	})
	t.Logf("files after empty: %v", files)
	if files > 3 {
		t.Fatalf("expected stale segments gone, got %v files", files)
	}
	for _, orphan := range []string{"000000200.dat", "000000201.dat", "000000201.dat.zstd"} {
		if _, err := os.Stat(filepath.Join(dataPath, orphan)); !os.IsNotExist(err) {
			t.Fatalf("orphan %v survived empty", orphan)
		}
	}
}
