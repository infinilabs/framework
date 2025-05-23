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

// Package grpcpool provides a pool of grpc clients
// https://github.com/processout/grpc-go-pool/blob/master/LICENSE
package rpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
)

var (
	// ErrClosed is the error when the client pool is closed
	ErrClosed = errors.New("grpc pool: client pool is closed")
	// ErrTimeout is the error when the client pool timed out
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	// ErrAlreadyClosed is the error when the client conn was already closed
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	// ErrFullPool is the error when the pool is already full
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
)

// Factory is a function type creating a grpc client
type Factory func() (*grpc.ClientConn, error)

// FactoryWithContext is a function type creating a grpc client
// that accepts the context parameter that could be passed from
// Get or NewWithContext method.
type FactoryWithContext func(context.Context) (*grpc.ClientConn, error)

// Pool is the grpc client pool
type Pool struct {
	clients         chan ClientConn
	factory         FactoryWithContext
	idleTimeout     time.Duration
	maxLifeDuration time.Duration
	mu              sync.RWMutex
}

// ClientConn is the wrapper for a grpc client conn
type ClientConn struct {
	*grpc.ClientConn
	pool          *Pool
	timeUsed      time.Time
	timeInitiated time.Time
	unhealthy     bool
}

// New creates a new clients pool with the given initial and maximum capacity,
// and the timeout for the idle clients. Returns an error if the initial
// clients could not be created
func New(factory Factory, init, capacity int, idleTimeout time.Duration,
	maxLifeDuration ...time.Duration) (*Pool, error) {
	return NewWithContext(context.Background(), func(ctx context.Context) (*grpc.ClientConn, error) { return factory() },
		init, capacity, idleTimeout, maxLifeDuration...)
}

// NewWithContext creates a new clients pool with the given initial and maximum
// capacity, and the timeout for the idle clients. The context parameter would
// be passed to the factory method during initialization. Returns an error if the
// initial clients could not be created.
func NewWithContext(ctx context.Context, factory FactoryWithContext, init, capacity int, idleTimeout time.Duration,
	maxLifeDuration ...time.Duration) (*Pool, error) {

	if capacity <= 0 {
		capacity = 1
	}
	if init < 0 {
		init = 0
	}
	if init > capacity {
		init = capacity
	}
	p := &Pool{
		clients:     make(chan ClientConn, capacity),
		factory:     factory,
		idleTimeout: idleTimeout,
	}
	if len(maxLifeDuration) > 0 {
		p.maxLifeDuration = maxLifeDuration[0]
	}
	for i := 0; i < init; i++ {
		c, err := factory(ctx)
		if err != nil {
			return nil, err
		}

		p.clients <- ClientConn{
			ClientConn:    c,
			pool:          p,
			timeUsed:      time.Now(),
			timeInitiated: time.Now(),
		}
	}
	// Fill the rest of the pool with empty clients
	for i := 0; i < capacity-init; i++ {
		p.clients <- ClientConn{
			pool: p,
		}
	}
	return p, nil
}

func (p *Pool) getClients() chan ClientConn {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.clients
}

// Close empties the pool calling Close on all its clients.
// You can call Close while there are outstanding clients.
// The pool channel is then closed, and Get will not be allowed anymore
func (p *Pool) Close() {
	p.mu.Lock()
	clients := p.clients
	p.clients = nil
	p.mu.Unlock()

	if clients == nil {
		return
	}

	close(clients)
	for client := range clients {
		if client.ClientConn == nil {
			continue
		}
		client.ClientConn.Close()
	}
}

// IsClosed returns true if the client pool is closed.
func (p *Pool) IsClosed() bool {
	return p == nil || p.getClients() == nil
}

// Get will return the next available client. If capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will wait till the next client becomes available or a timeout.
// A timeout of 0 is an indefinite wait
func (p *Pool) Get(ctx context.Context) (*ClientConn, error) {
	clients := p.getClients()
	if clients == nil {
		return nil, ErrClosed
	}

	wrapper := ClientConn{
		pool: p,
	}
	select {
	case wrapper = <-clients:
		// All good
	case <-ctx.Done():
		return nil, ErrTimeout // it would better returns ctx.Err()
	}

	// If the wrapper was idle too long, close the connection and create a new
	// one. It's safe to assume that there isn't any newer client as the client
	// we fetched is the first in the channel
	idleTimeout := p.idleTimeout
	if wrapper.ClientConn != nil && idleTimeout > 0 &&
		wrapper.timeUsed.Add(idleTimeout).Before(time.Now()) {

		wrapper.ClientConn.Close()
		wrapper.ClientConn = nil
	}

	var err error
	if wrapper.ClientConn == nil {
		wrapper.ClientConn, err = p.factory(ctx)
		if err != nil {
			// If there was an error, we want to put back a placeholder
			// client in the channel
			clients <- ClientConn{
				pool: p,
			}
		}
		// This is a new connection, reset its initiated time
		wrapper.timeInitiated = time.Now()
	}

	return &wrapper, err
}

// Unhealthy marks the client conn as unhealthy, so that the connection
// gets reset when closed
func (c *ClientConn) Unhealthy() {
	c.unhealthy = true
}

// Close returns a ClientConn to the pool. It is safe to call multiple time,
// but will return an error after first time
func (c *ClientConn) Close() error {
	if c == nil {
		return nil
	}
	if c.ClientConn == nil {
		return ErrAlreadyClosed
	}
	if c.pool.IsClosed() {
		return ErrClosed
	}
	// If the wrapper connection has become too old, we want to recycle it. To
	// clarify the logic: if the sum of the initialization time and the max
	// duration is before Now(), it means the initialization is so old adding
	// the maximum duration couldn't put in the future. This sum therefore
	// corresponds to the cut-off point: if it's in the future we still have
	// time, if it's in the past it's too old
	maxDuration := c.pool.maxLifeDuration
	if maxDuration > 0 && c.timeInitiated.Add(maxDuration).Before(time.Now()) {
		c.Unhealthy()
	}

	// We're cloning the wrapper so we can set ClientConn to nil in the one
	// used by the user
	wrapper := ClientConn{
		pool:       c.pool,
		ClientConn: c.ClientConn,
		timeUsed:   time.Now(),
	}
	if c.unhealthy {
		wrapper.ClientConn.Close()
		wrapper.ClientConn = nil
	} else {
		wrapper.timeInitiated = c.timeInitiated
	}
	select {
	case c.pool.clients <- wrapper:
		// All good
	default:
		return ErrFullPool
	}

	c.ClientConn = nil // Mark as closed
	return nil
}

// Capacity returns the capacity
func (p *Pool) Capacity() int {
	if p.IsClosed() {
		return 0
	}
	return cap(p.clients)
}

// Available returns the number of currently unused clients
func (p *Pool) Available() int {
	if p.IsClosed() {
		return 0
	}
	return len(p.clients)
}
