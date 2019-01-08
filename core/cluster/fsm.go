/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/cluster/raft"
	"io"
	"sync"
)

const Set string = "SET"
const Del string = "DEL"
const Incr string = "INC"
const Decr string = "DEC"

type MetadataFSM struct {
	l sync.Mutex
	m map[string]interface{}
}

func NewFSM() *MetadataFSM {
	return &MetadataFSM{m: make(map[string]interface{})}
}

// Get returns the value for the given key.
func (s *MetadataFSM) Get(key string) (interface{}, error) {
	s.l.Lock()
	defer s.l.Unlock()
	return s.m[key], nil
}

// Apply applies a Raft log entry to the key-value store.
func (f *MetadataFSM) Apply(l *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	log.Info("settting ,", c)

	switch c.Op {
	case Set:
		return f.applySet(c.Key, c.Value)
	case Del:
		return f.applyDelete(c.Key)
	case Incr:
		return f.applyIncr(c.Key)
	case Decr:
		return f.applyDecr(c.Key)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

func (f *MetadataFSM) applySet(key, value string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	f.m[key] = value
	return nil
}

func (f *MetadataFSM) applyDelete(key string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	delete(f.m, key)
	return nil
}

var step int64 = 1

func (f *MetadataFSM) applyIncr(key string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	f.m[key] = f.m[key].(int64) + step
	return nil
}

func (f *MetadataFSM) applyDecr(key string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	f.m[key] = f.m[key].(int64) - step
	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (f *MetadataFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.l.Lock()
	defer f.l.Unlock()

	// Clone the map.
	o := make(map[string]interface{})
	for k, v := range f.m {
		o[k] = v
	}
	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *MetadataFSM) Restore(rc io.ReadCloser) error {
	o := make(map[string]interface{})
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	log.Info("restore: ", o)

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.m = o
	return nil
}

type fsmSnapshot struct {
	store map[string]interface{}
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		if err := sink.Close(); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		sink.Cancel()
		return err
	}

	return nil
}

func (f *fsmSnapshot) Release() {}
