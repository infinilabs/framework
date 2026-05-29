package queue

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	corequeue "infini.sh/framework/core/queue"
)

func TestGetWriteTimeoutIncludesPayloadAndBacklog(t *testing.T) {
	dq := &DiskBasedQueue{
		cfg:       &DiskQueueConfig{WriteTimeoutInMS: defaultWriteTimeoutInMS},
		writeChan: make(chan []byte, defaultWriteChanBuffer),
	}

	dq.writeChan <- []byte("a")
	dq.writeChan <- []byte("b")

	timeout := dq.getWriteTimeout(3 * bytesPerMiB)

	expected := time.Duration(defaultWriteTimeoutInMS+3*adaptiveWriteTimeoutPerPayloadMiBInMS+2*adaptiveWriteTimeoutPerQueuedWriteInMS) * time.Millisecond
	if timeout != expected {
		t.Fatalf("unexpected write timeout: got %s want %s", timeout, expected)
	}
}

func TestGetWriteTimeoutCapsAtMaximum(t *testing.T) {
	dq := &DiskBasedQueue{
		cfg:       &DiskQueueConfig{WriteTimeoutInMS: defaultWriteTimeoutInMS},
		writeChan: make(chan []byte, defaultWriteChanBuffer),
	}

	for i := 0; i < cap(dq.writeChan); i++ {
		dq.writeChan <- []byte("x")
	}

	timeout := dq.getWriteTimeout(64 * bytesPerMiB)
	expected := time.Duration(maxAdaptiveWriteTimeoutInMS) * time.Millisecond
	if timeout != expected {
		t.Fatalf("unexpected capped timeout: got %s want %s", timeout, expected)
	}
}

func TestClosePersistsUnsyncedWritesBeforeClosingFiles(t *testing.T) {
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = t.TempDir()
	global.RegisterEnv(env1)

	queueName := "close-persists-unsynced"
	cfg := &DiskQueueConfig{
		MinMsgSize:       1,
		MaxMsgSize:       1024,
		MaxBytesPerFile:  1024 * 1024,
		SyncEveryRecords: 1 << 20,
		SyncTimeoutInMS:  1 << 20,
		ReadChanBuffer:   1,
		WriteChanBuffer:  1,
	}
	normalizeDiskQueueConfig(cfg)

	dataPath := GetDataPath(queueName)
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		t.Fatalf("failed to create queue data dir: %v", err)
	}

	dq := &DiskBasedQueue{
		name:               queueName,
		dataPath:           dataPath,
		cfg:                cfg,
		readChan:           make(chan []byte, cfg.ReadChanBuffer),
		depthChan:          make(chan int64),
		writeChan:          make(chan []byte, cfg.WriteChanBuffer),
		writeResponseChan:  make(chan WriteResponse),
		emptyChan:          make(chan int),
		emptyResponseChan:  make(chan error),
		exitChan:           make(chan int),
		exitSyncChan:       make(chan int, 1),
		consumersInReading: sync.Map{},
	}
	go dq.ioLoop()

	res := dq.Put([]byte("hello"))
	if res.Error != nil {
		t.Fatalf("failed to put queue message: %v", res.Error)
	}
	if dq.writeFile == nil {
		t.Fatalf("expected queue write file to remain open before close")
	}

	if err := dq.Close(); err != nil {
		t.Fatalf("failed to close queue: %v", err)
	}

	reopened := &DiskBasedQueue{
		name:               queueName,
		dataPath:           dataPath,
		cfg:                cfg,
		readChan:           make(chan []byte, cfg.ReadChanBuffer),
		depthChan:          make(chan int64),
		writeChan:          make(chan []byte, cfg.WriteChanBuffer),
		writeResponseChan:  make(chan WriteResponse),
		emptyChan:          make(chan int),
		emptyResponseChan:  make(chan error),
		exitChan:           make(chan int),
		exitSyncChan:       make(chan int, 1),
		consumersInReading: sync.Map{},
	}
	if err := reopened.retrieveMetaData(); err != nil {
		t.Fatalf("failed to reload queue metadata: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dataPath)
	})

	if depth := reopened.Depth(); depth != 1 {
		t.Fatalf("expected reopened queue depth 1, got %d", depth)
	}

	message, err := reopened.readOne()
	if err != nil {
		t.Fatalf("failed to read reopened queue message: %v", err)
	}
	if string(message) != "hello" {
		t.Fatalf("expected reopened queue payload %q, got %q", "hello", string(message))
	}
}

func TestResetOffsetSkipsMissingSegmentsUpToCurrentWriteSegment(t *testing.T) {
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = t.TempDir()
	global.RegisterEnv(env1)

	queueName := "reset-offset-skip"
	data := []byte("ok")
	fileName := GetFileName(queueName, 2)
	if err := os.MkdirAll(filepath.Dir(fileName), 0o755); err != nil {
		t.Fatalf("failed to create queue dir: %v", err)
	}
	file, err := os.Create(fileName)
	if err != nil {
		t.Fatalf("failed to create segment file: %v", err)
	}
	if err := binary.Write(file, binary.BigEndian, int32(len(data))); err != nil {
		t.Fatalf("failed to write message size: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatalf("failed to write message body: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("failed to close segment file: %v", err)
	}

	dq := &DiskBasedQueue{
		name:            queueName,
		cfg:             &DiskQueueConfig{AutoSkipCorruptFile: true, MinMsgSize: 1, MaxMsgSize: 1024},
		writeSegmentNum: 2,
		writePos:        int64(4 + len(data)),
	}
	consumer := &Consumer{
		ID:        "consumer-reset",
		diskQueue: dq,
		mCfg:      dq.cfg,
		qCfg:      &corequeue.QueueConfig{Name: queueName},
		cCfg:      &corequeue.ConsumerConfig{},
		queue:     queueName,
	}

	if err := consumer.ResetOffset(1, 0); err != nil {
		t.Fatalf("expected reset offset to skip to current write segment, got %v", err)
	}
	if consumer.segment != 2 {
		t.Fatalf("expected consumer to move to segment 2, got %d", consumer.segment)
	}
	if consumer.reader == nil {
		t.Fatalf("expected consumer reader to be initialized for segment 2")
	}
}

func TestFetchMessagesRecoversToEmptyTailWithoutRescanningCorruptFile(t *testing.T) {
	env1 := EmptyEnv()
	env1.SystemConfig.PathConfig.Data = t.TempDir()
	global.RegisterEnv(env1)

	queueName := "fetch-empty-tail"
	corruptFile := GetFileName(queueName, 1)
	if err := os.MkdirAll(filepath.Dir(corruptFile), 0o755); err != nil {
		t.Fatalf("failed to create queue dir: %v", err)
	}
	if err := os.WriteFile(corruptFile, []byte{0x7f, 0xff, 0xff, 0xff}, 0o644); err != nil {
		t.Fatalf("failed to write corrupt segment: %v", err)
	}

	dq := &DiskBasedQueue{
		name:            queueName,
		cfg:             &DiskQueueConfig{AutoSkipCorruptFile: true, MinMsgSize: 1, MaxMsgSize: 1024},
		writeSegmentNum: 3,
		writePos:        0,
	}
	consumer := &Consumer{
		ID:        "consumer-fetch",
		diskQueue: dq,
		mCfg:      dq.cfg,
		qCfg:      &corequeue.QueueConfig{Name: queueName},
		cCfg:      &corequeue.ConsumerConfig{},
		queue:     queueName,
	}

	if err := consumer.ResetOffset(1, 0); err != nil {
		t.Fatalf("failed to initialize consumer: %v", err)
	}

	ctx := &corequeue.Context{}
	messages, timeout, err := consumer.FetchMessages(ctx, 1)
	if err != nil {
		t.Fatalf("expected corruption recovery without error, got %v", err)
	}
	if timeout {
		t.Fatalf("did not expect timeout during corruption recovery")
	}
	if len(messages) != 0 {
		t.Fatalf("expected no messages during recovery, got %d", len(messages))
	}
	if consumer.segment != dq.writeSegmentNum {
		t.Fatalf("expected consumer to park on new tail segment %d, got %d", dq.writeSegmentNum, consumer.segment)
	}
	if ctx.NextOffset.Segment != dq.writeSegmentNum || ctx.NextOffset.Position != 0 {
		t.Fatalf("expected next offset to advance to new tail, got %v", ctx.NextOffset)
	}

	payload := []byte("hello")
	tailFile := GetFileName(queueName, dq.writeSegmentNum)
	file, err := os.Create(tailFile)
	if err != nil {
		t.Fatalf("failed to create new tail segment: %v", err)
	}
	if err := binary.Write(file, binary.BigEndian, int32(len(payload))); err != nil {
		t.Fatalf("failed to write tail message size: %v", err)
	}
	if _, err := file.Write(payload); err != nil {
		t.Fatalf("failed to write tail message body: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("failed to close tail segment: %v", err)
	}
	dq.writePos = int64(4 + len(payload))

	ctx = &corequeue.Context{}
	messages, timeout, err = consumer.FetchMessages(ctx, 1)
	if err != nil {
		t.Fatalf("expected consumer to resume reading on new tail, got %v", err)
	}
	if timeout {
		t.Fatalf("did not expect timeout when new tail data exists")
	}
	if len(messages) != 1 {
		t.Fatalf("expected exactly one message, got %d", len(messages))
	}
	if string(messages[0].Data) != "hello" {
		t.Fatalf("expected payload %q, got %q", "hello", string(messages[0].Data))
	}
}
