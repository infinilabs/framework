// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/kversion"
	"infini.sh/framework/core/queue"
)

// commitProtocolBroker models persisted offsets and configurable commit responses.
//
// The real SDK talks to a local TCP server so tests can separate transport success
// from partition failure, missing response entries, and delayed completion.
type commitProtocolBroker struct {
	mu              sync.Mutex
	offsets         map[int32]int64
	beforeCommit    func(*kmsg.OffsetCommitRequest)
	errorCode       int16
	partitionErrors map[int32]int16
	omit            map[int32]bool
	requestCount    int
}

// Helper function to connect the real SDK to a local commit-protocol fixture.
func newCommitProtocolClient(t *testing.T, b *commitProtocolBroker) *kgo.Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := int32(ln.Addr().(*net.TCPAddr).Port)
	var mu sync.Mutex
	conns := map[net.Conn]struct{}{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns[conn] = struct{}{}
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
						t.Error(err)
					}
				}()
				defer func() { mu.Lock(); delete(conns, conn); mu.Unlock() }()
				for {
					var size [4]byte
					if _, err := io.ReadFull(conn, size[:]); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(size[:])
					if n < 10 || n > 1<<20 {
						t.Errorf("invalid fixture request size %d", n)
						return
					}
					data := make([]byte, n)
					if _, err := io.ReadFull(conn, data); err != nil {
						return
					}
					key := int16(binary.BigEndian.Uint16(data[0:2]))
					version := int16(binary.BigEndian.Uint16(data[2:4]))
					headerLen := 10
					clientLen := int(int16(binary.BigEndian.Uint16(data[8:10])))
					if clientLen > 0 {
						headerLen += clientLen
					}
					req := kmsg.RequestForKey(key)
					if req == nil {
						t.Errorf("unknown request key %d", key)
						return
					}
					req.SetVersion(version)
					if err := req.ReadFrom(data[headerLen:]); err != nil {
						t.Errorf("decode request %d: %v", key, err)
						return
					}
					resp := req.ResponseKind()
					resp.SetVersion(version)
					switch r := req.(type) {
					case *kmsg.MetadataRequest:
						response := resp.(*kmsg.MetadataResponse)
						response.Brokers = []kmsg.MetadataResponseBroker{{NodeID: 1, Host: "127.0.0.1", Port: port}}
					case *kmsg.FindCoordinatorRequest:
						response := resp.(*kmsg.FindCoordinatorResponse)
						response.NodeID = 1
						response.Host = "127.0.0.1"
						response.Port = port
					case *kmsg.LeaveGroupRequest:
						// The default response acknowledges client cleanup.
					case *kmsg.OffsetCommitRequest:
						if b.beforeCommit != nil {
							b.beforeCommit(r)
						}
						response := resp.(*kmsg.OffsetCommitResponse)
						b.mu.Lock()
						b.requestCount++
						for _, topic := range r.Topics {
							out := kmsg.OffsetCommitResponseTopic{Topic: topic.Topic}
							for _, part := range topic.Partitions {
								code := b.errorCode
								if v, ok := b.partitionErrors[part.Partition]; ok {
									code = v
								}
								if code == 0 {
									b.offsets[part.Partition] = part.Offset
								}
								if !b.omit[part.Partition] {
									out.Partitions = append(out.Partitions, kmsg.OffsetCommitResponseTopicPartition{Partition: part.Partition, ErrorCode: code})
								}
							}
							response.Topics = append(response.Topics, out)
						}
						b.mu.Unlock()
					default:
						t.Errorf("unexpected fixture request %T", req)
						return
					}
					payload := append([]byte(nil), data[4:8]...)
					payload = resp.AppendTo(payload)
					binary.BigEndian.PutUint32(size[:], uint32(len(payload)))
					if _, err := conn.Write(append(size[:], payload...)); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
		mu.Lock()
		for c := range conns {
			if err := c.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		}
		mu.Unlock()
		wg.Wait()
	})
	client, err := kgo.NewClient(kgo.SeedBrokers(ln.Addr().String()), kgo.MaxVersions(kversion.V0_9_0()), kgo.ConsumerGroup("commit-tests_test-topic"), kgo.DisableAutoCommit())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

// A successful late response cannot restore a revoked processing session.
func TestCommitOffset_DoesNotRestoreRevokedPartition(t *testing.T) {
	c := newTestConsumer()
	appendRecords(t, c, 1, 0, 10)
	appendRecords(t, c, 3, 0, 10)
	b := &commitProtocolBroker{offsets: map[int32]int64{}}
	b.beforeCommit = func(*kmsg.OffsetCommitRequest) { c.onRevoked(context.Background(), nil, nil) }
	c.client = newCommitProtocolClient(t, b)
	defer closeTestConsumer(t, c)
	if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 10, c.version)); err == nil {
		t.Fatal("revoked session reported successful confirmation")
	}
	if len(c.order.base) != 0 {
		t.Fatal("late response restored committed state")
	}
	if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 10, c.version)); err == nil {
		t.Fatal("expired session accepted another confirmation")
	}
}

// A cross-partition final commit persists the whole completed prefix.
func TestCommitOffset_CrossPartitionChunkPreservesProgress(t *testing.T) {
	c := newTestConsumer()
	appendRecords(t, c, 1, 0, 10)
	appendRecords(t, c, 3, 0, 10)
	b := &commitProtocolBroker{offsets: map[int32]int64{}}
	c.client = newCommitProtocolClient(t, b)
	defer closeTestConsumer(t, c)
	if err := c.CommitOffset(queue.NewOffsetWithVersion(1, 5, c.version)); err != nil {
		t.Fatal(err)
	}
	if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 10, c.version)); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.offsets[1] != 10 || b.offsets[3] != 10 {
		t.Fatal(b.offsets)
	}
}

// The real SDK can return a nil transport error while a partition rejects its commit.
func TestCommitOffset_PartitionFailure(t *testing.T) {
	c := newTestConsumer()
	appendRecords(t, c, 1, 0, 10)
	b := &commitProtocolBroker{offsets: map[int32]int64{}, errorCode: 30}
	c.client = newCommitProtocolClient(t, b)
	defer closeTestConsumer(t, c)
	if err := c.CommitOffset(queue.NewOffsetWithVersion(1, 10, c.version)); err == nil {
		t.Fatal("partition failure was reported as success")
	}
	if len(c.order.base) != 0 {
		t.Fatal("failed prefix was retired")
	}
}

// Closing a session removes its routing entry and never forwards a delayed acknowledgement.
func TestKafkaQueueCommitOffset_ClosedSession(t *testing.T) {
	c := newTestConsumer()
	q := &KafkaQueue{}
	c.owner = q
	q.sessions.Store(c.version, c)
	q.consumers.Store(getGroupForKafka(c.cCfg.Group, c.qCfg.ID), c)
	c.leased = true
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if ok, err := q.CommitOffset(c.qCfg, c.cCfg, queue.NewOffsetWithVersion(1, 1, c.version)); ok || err == nil {
		t.Fatal("closed session acknowledgement accepted")
	}
}

// Partial and incomplete broker responses never retire the business prefix.
func TestCommitOffsetIncompleteResponse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		codes map[int32]int16
		omit  map[int32]bool
	}{
		{"partial-success", map[int32]int16{3: 30}, nil},
		{"missing-partition", nil, map[int32]bool{3: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestConsumer()
			appendRecords(t, c, 1, 0, 2)
			appendRecords(t, c, 3, 0, 2)
			b := &commitProtocolBroker{offsets: map[int32]int64{}, partitionErrors: tc.codes, omit: tc.omit}
			c.client = newCommitProtocolClient(t, b)
			defer closeTestConsumer(t, c)
			if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 2, c.version)); err == nil {
				t.Fatal("incomplete response accepted")
			}
			if len(c.order.base) != 0 {
				t.Fatal("failed prefix was retired")
			}
			if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 2, c.version)); err == nil {
				t.Fatal("failed session continued")
			}
		})
	}
}

// The request hook rejects a different SDK identity before any commit reaches the broker.
func TestCommitOffsetWrongIdentity(t *testing.T) {
	c := newTestConsumer()
	appendRecords(t, c, 1, 0, 1)
	b := &commitProtocolBroker{offsets: map[int32]int64{}}
	c.client = newCommitProtocolClient(t, b)
	defer closeTestConsumer(t, c)
	c.generation = 7
	if err := c.CommitOffset(queue.NewOffsetWithVersion(1, 1, c.version)); err == nil {
		t.Fatal("different generation accepted")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.requestCount != 0 {
		t.Fatal("wrong generation reached the broker")
	}
}

// Closing cancels an in-flight network commit before waiting for the operation lock.
func TestCloseDuringCommit(t *testing.T) {
	c := newTestConsumer()
	appendRecords(t, c, 1, 0, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	b := &commitProtocolBroker{offsets: map[int32]int64{}, beforeCommit: func(*kmsg.OffsetCommitRequest) { close(entered); <-release }}
	c.client = newCommitProtocolClient(t, b)
	done := make(chan error, 1)
	go func() { done <- c.CommitOffset(queue.NewOffsetWithVersion(1, 1, c.version)) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("commit did not reach the broker")
	}
	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close waited for the blocked response")
	}
	if err := <-done; err == nil {
		t.Fatal("cancelled commit reported success")
	}
}

// An administrative offset cannot enter the business acknowledgement path.
func TestCommitOffset_RejectsAdministrativeOffset(t *testing.T) {
	c := newTestConsumer()
	defer closeTestConsumer(t, c)
	appendRecords(t, c, 1, 0, 10)
	appendRecords(t, c, 3, 0, 10)
	if err := c.CommitOffset(queue.NewOffsetWithVersion(3, 10, 1)); err == nil {
		t.Fatal("administrative reset accepted as business confirmation")
	}
	if c.order.base[1] != 0 || c.order.base[3] != 0 {
		t.Fatal("admin request changed business progress")
	}
}
