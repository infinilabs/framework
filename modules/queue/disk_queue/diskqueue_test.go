package queue

import (
	"testing"
	"time"
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
