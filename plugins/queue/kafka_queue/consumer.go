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
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/queue"
)

// consumerPhase separates an instance's read mode from its ability to accept operations.
//
// An assignment belongs to one session. Rebalance or failure ends that session,
// so the same instance cannot resume with a different set of commit targets.
type consumerPhase uint8

const (
	// consumerPrepared has no SDK client. The first read or reset selects its mode.
	consumerPrepared consumerPhase = iota
	// consumerProcessing uses a group client. Its first assignment can still be pending.
	consumerProcessing
	// consumerBrowsing selects direct reads without group membership or business commits.
	consumerBrowsing
	// consumerExpired rejects further reads and commits. Close must release its resources.
	consumerExpired
	// consumerClosed permanently rejects operations. Close can still retry failed lease cleanup.
	consumerClosed
)

// sessionSequence allocates process-local identities. Their negative values distinguish
// business acknowledgements from nonnegative administrative offsets.
var sessionSequence atomic.Int64

// Consumer owns one delivery session, its configuration snapshots, and one SDK client.
//
// A physical partition offset alone cannot identify a completed prefix across partitions.
// The session retains delivery order until explicit confirmation succeeds.
//
// opMu serializes reads, commits, and cleanup. mu protects phase, cause, and assignment
// identity so SDK callbacks can cancel operations without waiting for opMu.
type Consumer struct {
	owner *KafkaQueue
	qCfg  *queue.QueueConfig
	cCfg  *queue.ConsumerConfig

	// opMu protects client creation, delivery order, read mode setup, and lease flags.
	client *kgo.Client
	opMu   sync.Mutex
	mu     sync.Mutex
	phase  consumerPhase
	cause  error // First expiration cause, retained until the instance is closed.

	// version is immutable. Callers must preserve it in the complete NextOffset.
	version int64
	ioCtx   context.Context // Canceled on expiration, including a concurrent Close.
	cancel  context.CancelFunc

	// mu protects this identity from the first assignment. It is never replaced.
	assigned   bool
	memberID   string
	generation int32

	order           deliveryOrder // In-memory delivery history, never durable recovery state.
	browsePartition int32         // Physical address selected before the first read.
	browsePosition  int64
	leased          bool // This instance owns a local slot that still needs cleanup.
	lockHeld        bool // This instance also acquired the external framework lease.
}

// Helper function to invalidate a session and cancel its pending I/O.
//
// Assignment callbacks cannot wait for opMu or close the SDK while an operation
// waits on that callback. Close performs resource cleanup later.
func (c *Consumer) expire(err error) {
	c.mu.Lock()
	if c.phase != consumerClosed && c.phase != consumerExpired {
		c.phase = consumerExpired
		c.cause = err
		if c.cancel != nil {
			c.cancel()
		}
	}
	c.mu.Unlock()
}

// Helper function to reject expired or closed sessions during reads, commits, and setup.
//
// Callbacks change lifecycle state independently of opMu. Each operation must
// observe that change before publishing records or accepting a commit result.
func (c *Consumer) stateError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase == consumerClosed {
		return fmt.Errorf("kafka consumer is closed")
	}
	if c.phase == consumerExpired {
		return fmt.Errorf("kafka consumer session expired: %w", c.cause)
	}
	return nil
}

// Helper function to renew the framework lease before reads and confirmations.
//
// Kafka group membership does not replace the framework's ownership rule.
// Browsing has no lease. A failed renewal prevents further business progress.
func (c *Consumer) renewLease() error {
	if !c.lockHeld {
		return nil
	}
	key := getGroupForKafka(c.cCfg.Group, c.qCfg.ID)
	ok, err := locker.Hold(queue.BucketWhoOwnsThisTopic, key, global.Env().SystemConfig.NodeConfig.ID, 60*time.Second, true)
	if err != nil || !ok {
		return fmt.Errorf("kafka consumer lease was lost: %s: %v", key, err)
	}
	return nil
}

// onAssigned captures the first Kafka member identity for this session.
//
// Delivery history must belong to one assignment. A subsequent assignment expires
// the instance instead of attaching old acknowledgements to the new assignment.
func (c *Consumer) onAssigned(_ context.Context, client *kgo.Client, _ map[string][]int32) {
	member, generation := client.GroupMetadata()
	c.mu.Lock()
	if c.phase != consumerProcessing {
		c.mu.Unlock()
		return
	}
	if c.assigned {
		c.mu.Unlock()
		c.expire(fmt.Errorf("kafka assignment changed"))
		return
	}
	c.assigned = true
	c.memberID = member
	c.generation = generation
	c.mu.Unlock()
}

// onRevoked ends a session once Kafka withdraws its established assignment.
//
// The SDK can call this before the first assignment. That initial callback has
// no delivery history to invalidate and does not prevent startup.
func (c *Consumer) onRevoked(_ context.Context, _ *kgo.Client, _ map[string][]int32) {
	c.mu.Lock()
	assigned := c.assigned
	c.mu.Unlock()
	if assigned {
		c.expire(fmt.Errorf("kafka assignment was revoked"))
	}
}

// openProcessing reserves ownership and creates the group client on the first business read.
//
// Delayed creation lets ResetOffset select browsing without joining the business group.
// Only explicit confirmation can commit, so SDK auto-commit is always disabled.
//
// FetchMessages holds opMu. If setup fails after reserving ownership, Close must
// release it. Poll protection covers recording deliveries before rebalance can proceed.
func (c *Consumer) openProcessing() error {
	if err := c.owner.claim(c); err != nil {
		return err
	}
	c.leased = true
	key := getGroupForKafka(c.cCfg.Group, c.qCfg.ID)
	ok, err := locker.Hold(queue.BucketWhoOwnsThisTopic, key, global.Env().SystemConfig.NodeConfig.ID, 60*time.Second, true)
	if err != nil || !ok {
		return fmt.Errorf("kafka consumer already owning this topic: %s: %v", key, err)
	}
	c.lockHeld = true
	c.mu.Lock()
	if c.phase != consumerPrepared {
		c.mu.Unlock()
		return c.stateError()
	}
	c.phase = consumerProcessing
	c.mu.Unlock()
	opts := []kgo.Opt{
		kgo.ConsumerGroup(key), kgo.ConsumeTopics(c.qCfg.ID), kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.RangeBalancer()), kgo.BlockRebalanceOnPoll(),
		kgo.OnPartitionsAssigned(c.onAssigned), kgo.OnPartitionsRevoked(c.onRevoked),
		kgo.OnPartitionsLost(func(context.Context, *kgo.Client, map[string][]int32) {
			c.expire(fmt.Errorf("kafka group session was lost"))
		}),
		kgo.SessionTimeout(60 * time.Second), kgo.HeartbeatInterval(10 * time.Second),
		kgo.RetryTimeout(10 * time.Second), kgo.RebalanceTimeout(10 * time.Second),
	}
	if c.cCfg.AutoResetOffset == "latest" {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	} else {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	}
	if c.cCfg.FetchMinBytes > 0 {
		opts = append(opts, kgo.FetchMinBytes(int32(c.cCfg.FetchMinBytes)))
	}
	if c.cCfg.FetchMaxBytes > 0 {
		opts = append(opts, kgo.FetchMaxBytes(int32(c.cCfg.FetchMaxBytes)))
	}
	if c.cCfg.FetchMaxWaitMs > 0 {
		opts = append(opts, kgo.FetchMaxWait(time.Duration(c.cCfg.FetchMaxWaitMs)*time.Millisecond))
	}
	c.client = c.owner.newClient(opts)
	return nil
}

// ResetOffset selects a physical partition and position for direct, read-only browsing.
//
// Browsing must not join the business group or change its recovery position.
// The first FetchMessages checks the retained range before creating the direct client.
//
// Reset is allowed only before the first read attempt. An invalid or late reset
// expires the instance. This method does not reset a group's committed offsets.
func (c *Consumer) ResetOffset(partition, position int64) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	if err := c.stateError(); err != nil {
		return err
	}
	if c.client != nil || partition < 0 || partition > math.MaxInt32 || position < 0 {
		err := fmt.Errorf("kafka ResetOffset requires a valid position before the first fetch")
		c.expire(err)
		return err
	}
	c.mu.Lock()
	c.phase = consumerBrowsing
	c.browsePartition = int32(partition)
	c.browsePosition = position
	c.mu.Unlock()
	return nil
}

// FetchMessages returns records in delivery order and fills ctx with the returned batch boundaries.
//
// Business confirmation can span reads or end inside a batch. The session records
// delivery order so a complete NextOffset identifies the exact prefix to confirm.
//
// count and FetchMaxMessages limit returned records. FetchMaxBytes is a soft limit.
// An empty deadline returns timeout=true. A real fetch or recording failure expires
// the session and discards the unpublished batch without committing it.
func (c *Consumer) FetchMessages(ctx *queue.Context, count int) ([]queue.Message, bool, error) {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	if ctx == nil {
		return nil, false, fmt.Errorf("nil Kafka queue context")
	}
	*ctx = queue.Context{}
	if err := c.stateError(); err != nil {
		return nil, false, err
	}
	c.mu.Lock()
	phase := c.phase
	c.mu.Unlock()
	if c.client == nil {
		if phase == consumerBrowsing {
			starts, ends, err := c.owner.positionBounds(c.ioCtx, c.qCfg.ID)
			if err != nil {
				c.expire(err)
				return nil, false, err
			}
			start, ok := starts[c.browsePartition]
			end := ends[c.browsePartition]
			if !ok || c.browsePosition < start || c.browsePosition > end {
				err = fmt.Errorf("kafka browse position is out of range")
				c.expire(err)
				return nil, false, err
			}
			c.client = c.owner.newClient([]kgo.Opt{kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{c.qCfg.ID: {c.browsePartition: kgo.NewOffset().At(c.browsePosition)}}), kgo.ConsumeResetOffset(kgo.NoResetOffset())})
		} else if err := c.openProcessing(); err != nil {
			c.expire(err)
			return nil, false, err
		}
	}
	if err := c.stateError(); err != nil {
		return nil, false, err
	}
	if err := c.renewLease(); err != nil {
		c.expire(err)
		return nil, false, err
	}
	wait := time.Duration(c.cCfg.FetchMaxWaitMs) * time.Millisecond
	if wait <= 0 {
		wait = time.Second
	}
	pollCtx, cancel := context.WithTimeout(c.ioCtx, wait)
	defer cancel()
	limit := count
	if limit <= 0 {
		limit = c.cCfg.FetchMaxMessages
	}
	if limit <= 0 {
		limit = 1000
	}
	if c.cCfg.FetchMaxMessages > 0 && limit > c.cCfg.FetchMaxMessages {
		limit = c.cCfg.FetchMaxMessages
	}
	messages := make([]queue.Message, 0)
	bytes := 0
	for len(messages) < limit {
		fetches := c.client.PollRecords(pollCtx, limit-len(messages))
		err := func() error {
			// Poll may hold rebalance protection even on a timeout or an empty result.
			defer c.client.AllowRebalance()
			if err := c.stateError(); err != nil {
				return err
			}
			for _, failure := range fetches.Errors() {
				if errors.Is(failure.Err, context.DeadlineExceeded) && pollCtx.Err() == context.DeadlineExceeded {
					continue
				}
				return fmt.Errorf("kafka fetch %s[%d]: %w", failure.Topic, failure.Partition, failure.Err)
			}
			records := fetches.Records()
			for _, r := range records {
				if r.Partition < 0 || r.Offset < 0 || r.Offset == math.MaxInt64 {
					return fmt.Errorf("invalid Kafka record address")
				}
			}
			if phase != consumerBrowsing {
				if err := c.order.append(records); err != nil {
					return err
				}
			}
			version := c.version
			if phase == consumerBrowsing {
				version = 0
			}
			for _, r := range records {
				off := queue.NewOffsetWithVersion(int64(r.Partition), r.Offset, version)
				next := queue.NewOffsetWithVersion(int64(r.Partition), r.Offset+1, version)
				messages = append(messages, queue.Message{Offset: off, NextOffset: next, Data: r.Value, Size: len(r.Value), Timestamp: r.Timestamp.Unix()})
				bytes += len(r.Value)
			}
			return nil
		}()
		if err != nil {
			c.expire(err)
			return nil, false, err
		}
		if pollCtx.Err() != nil || (c.cCfg.FetchMaxBytes > 0 && bytes >= c.cCfg.FetchMaxBytes) {
			break
		}
	}
	if err := c.stateError(); err != nil {
		return nil, false, err
	}
	if len(messages) == 0 {
		return nil, true, nil
	}
	ctx.MessageCount = len(messages)
	ctx.InitOffset = messages[0].Offset
	ctx.NextOffset = messages[len(messages)-1].NextOffset
	return messages, false, nil
}

// CommitOffset confirms all messages delivered through the supplied NextOffset in this session.
//
// Partition offsets are independent. The delivery history converts this single
// boundary into fixed commit positions for every partition in the completed prefix.
//
// The complete offset must retain its negative session Version. A covered prefix
// is a no-op. Only complete success retires history. SDK commit errors or incomplete
// responses end the session. Recovery can repeat messages from Kafka's saved positions.
func (c *Consumer) CommitOffset(off queue.Offset) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	if err := c.stateError(); err != nil {
		return err
	}
	c.mu.Lock()
	valid := c.phase == consumerProcessing && c.assigned && off.Version == c.version
	member, generation := c.memberID, c.generation
	c.mu.Unlock()
	if !valid || off.Segment < 0 || off.Segment > math.MaxInt32 {
		return fmt.Errorf("kafka acknowledgement does not belong to this processing session")
	}
	if err := c.renewLease(); err != nil {
		c.expire(err)
		return err
	}
	targets, covered, err := c.order.target(int32(off.Segment), off.Position)
	if err != nil {
		c.expire(err)
		return err
	}
	if covered {
		return nil
	}
	req := map[string]map[int32]kgo.EpochOffset{c.qCfg.ID: {}}
	for p, n := range targets {
		req[c.qCfg.ID][p] = kgo.EpochOffset{Offset: n, Epoch: -1}
	}
	ctx, cancel := context.WithTimeout(c.ioCtx, 10*time.Second)
	defer cancel()
	// The SDK builds requests from its current group metadata. Check the assembled
	// request so a changed identity cannot commit targets from the old assignment.
	ctx = kgo.PreCommitFnContext(ctx, func(request *kmsg.OffsetCommitRequest) error {
		if err := c.stateError(); err != nil {
			return err
		}
		if request.Group != getGroupForKafka(c.cCfg.Group, c.qCfg.ID) || request.MemberID != member || request.Generation != generation {
			return fmt.Errorf("kafka commit identity changed before sending")
		}
		return nil
	})
	// A nil transport error does not prove that every partition accepted its target.
	// Partial success can change broker state, but must not retire local history.
	var result error
	c.client.CommitOffsetsSync(ctx, req, func(_ *kgo.Client, _ *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
		result = err
		if result != nil {
			return
		}
		if response == nil {
			result = fmt.Errorf("kafka commit response is missing")
			return
		}
		seen := make(map[int32]bool, len(targets))
		for _, topic := range response.Topics {
			if topic.Topic != c.qCfg.ID {
				result = fmt.Errorf("unexpected Kafka commit response topic")
				return
			}
			for _, p := range topic.Partitions {
				if _, ok := targets[p.Partition]; !ok || seen[p.Partition] {
					result = fmt.Errorf("unexpected Kafka commit response partition")
					return
				}
				seen[p.Partition] = true
				if p.ErrorCode != 0 {
					result = fmt.Errorf("kafka commit partition %d: %w", p.Partition, kerr.ErrorForCode(p.ErrorCode))
					return
				}
			}
		}
		if len(seen) != len(targets) {
			result = fmt.Errorf("kafka commit response omitted a partition")
		}
	})
	if result != nil {
		c.expire(result)
		return result
	}
	if err := c.stateError(); err != nil {
		return err
	}
	c.order.retire(int32(off.Segment), off.Position, targets)
	return nil
}

// Close cancels pending operations, closes the SDK client, and releases this instance's ownership.
//
// Cancellation must precede waiting for opMu because a read or commit can block
// while holding it. SDK shutdown precedes lease release so a replacement cannot
// overlap with the old client.
//
// Closed means business operations are disabled. A failed lease release retains
// the local slot and registry entry, and a later Close retries that cleanup.
func (c *Consumer) Close() error {
	c.expire(fmt.Errorf("kafka consumer was closed"))
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.Lock()
	closed := c.phase == consumerClosed
	c.phase = consumerClosed
	c.mu.Unlock()
	if !closed {
		if c.client != nil {
			c.client.CloseAllowingRebalance()
		}
		c.order = deliveryOrder{}
	}
	if c.owner != nil && c.leased {
		if err := c.owner.unclaim(c); err != nil {
			return err
		}
	}
	if c.owner != nil {
		c.owner.instances.Delete(c)
	}
	return nil
}
