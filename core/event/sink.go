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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package event

import (
	"time"
)

// EventSink defines the interface for outputting collected events.
// Implementations can write to different destinations (queue, file, etc.)
type EventSink interface {
	// Save writes an event with the current timestamp
	Save(event *Event) error
	// SaveWithTimestamp writes an event with a specific timestamp
	SaveWithTimestamp(event *Event, ts time.Time) error
}

// QueueEventSink implements EventSink by pushing events to the default queue.
// This preserves the existing behavior for pipeline mode.
type QueueEventSink struct{}

// DefaultEventSink is the global default sink that uses the queue
var DefaultEventSink EventSink = &QueueEventSink{}

func (s *QueueEventSink) Save(event *Event) error {
	return Save(event)
}

func (s *QueueEventSink) SaveWithTimestamp(event *Event, ts time.Time) error {
	return SaveWithTimestamp(event, ts)
}
