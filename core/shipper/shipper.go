/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package shipper defines the direct-ship registry: producers that have
// their own durable source (e.g. the agent's file collector, where the
// file itself plus offset checkpoints provide durability) can bypass the
// local queue and hand envelopes straight to a registered shipper.
//
// Implementations live outside this package and register a Factory via
// Register, typically from an init(); products activate them with a
// blank-import (the same pattern as pipeline processor registration).
package shipper

import (
	"fmt"
	"sort"
	"sync"
)

// Shipper delivers batches of serialized log-event envelopes (the otel
// envelope JSON also used at the queue boundary).
//
// Ship returns an error when the batch was NOT delivered; the caller
// must then treat the events as undelivered -- for file sources that
// means keeping the offset at the last committed position so the source
// is re-read.
type Shipper interface {
	// Ship delivers one batch. The batch is only mutated by the caller
	// after Ship returns.
	Ship(batch [][]byte) error

	// Close releases the shipper's resources.
	Close() error
}

// Factory builds a Shipper from a configuration map (structure defined
// by the implementation).
type Factory func(cfg map[string]interface{}) (Shipper, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register adds a shipper factory under the given name. Intended for
// package init(); a duplicate name panics to surface wiring mistakes.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("shipper factory with same name already exists: %v", name))
	}
	factories[name] = f
}

// Get builds the named shipper.
func Get(name string, cfg map[string]interface{}) (Shipper, error) {
	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no shipper registered under name %q (available: %v)", name, Names())
	}
	return f(cfg)
}

// Names lists the registered shipper names (sorted, for errors and UI).
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
