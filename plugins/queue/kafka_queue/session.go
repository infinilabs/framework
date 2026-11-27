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
	"fmt"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/queue"
)

// Helper function to reserve one local business slot and register its acknowledgement identity.
//
// A backend allows one business instance per logical group and topic. Reserving
// before external lease acquisition also excludes local administrative resets.
//
// openProcessing holds Consumer.opMu. This function acquires KafkaQueue.mu and
// rejects new claims after Stop closes the creation path.
func (q *KafkaQueue) claim(c *Consumer) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopping {
		return fmt.Errorf("kafka queue is stopping")
	}
	group := getGroupForKafka(c.cCfg.Group, c.qCfg.ID)
	if _, loaded := q.consumers.LoadOrStore(group, c); loaded {
		return fmt.Errorf("kafka consumer already owning this topic: %s", group)
	}
	q.sessions.Store(c.version, c)
	return nil
}

// Helper function to remove this instance's slot and acknowledgement route after lease release.
//
// The external lease identifies the node, not the consumer instance. Releasing it
// after admitting a replacement on the same node can delete the replacement's lease.
//
// Close holds Consumer.opMu and shuts down the SDK first. This function holds
// KafkaQueue.mu across release. Failure retains the slot and route for a later Close.
func (q *KafkaQueue) unclaim(c *Consumer) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	group := getGroupForKafka(c.cCfg.Group, c.qCfg.ID)
	value, ok := q.consumers.Load(group)
	if !ok || value != c {
		return fmt.Errorf("kafka consumer lease no longer belongs to this instance")
	}
	if c.lockHeld {
		if err := locker.Release(queue.BucketWhoOwnsThisTopic, group, global.Env().SystemConfig.NodeConfig.ID); err != nil {
			return err
		}
	}
	q.consumers.Delete(group)
	q.sessions.Delete(c.version)
	c.leased = false
	return nil
}
