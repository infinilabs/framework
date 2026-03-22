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

/*
Package queue provides a pluggable queue abstraction layer supporting both
simple push/pop and advanced consumer-group-based queue implementations.

The [QueueAPI] interface combines [SimpleQueueAPI] for basic operations
with [AdvancedQueueAPI] for offset management and consumer groups. Use
[Register] or [RegisterDefaultHandler] to plug in a queue backend, and
[IniQueue] to initialize a named queue. Producers and consumers are
obtained via [AcquireProducer] and [AcquireConsumer].
*/
package queue
