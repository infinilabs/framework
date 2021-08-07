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
	"github.com/segmentio/encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/cluster/raft"
	"infini.sh/framework/core/util"
	"io"
	"sync"
)

const NodeUp string = "NODE_UP"
const NodeDown string = "NODE_DOWN"
const NodeLeave string = "NODE_LEAVE"

type ClusterFSM struct {
	l    sync.Mutex
	meta Metadata
}

func NewFSM() *ClusterFSM {
	return &ClusterFSM{
		meta: Metadata{KnownNodesRPCEndpoint: make(map[string]*Node)},
	}
}


// Apply applies a Raft log entry to the key-value store.
func (f *ClusterFSM) Apply(l *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}
	switch c.Op {
	case NodeUp:
		node := Node{}
		err:=util.FromJson(c.Value, &node)
		if err!=nil{
			panic(err)
		}
		return f.applyNodeUp(c.Key, &node)
	case NodeDown:
		return f.applyNodeDown(c.Key)
	case NodeLeave:
		return f.applyNodeLeave(c.Key)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

func (f *ClusterFSM) GetClusterMetadata() Metadata {
	return util.DeepCopy(f.meta).(Metadata)
}

func (f *ClusterFSM) applyNodeUp(key string, node *Node) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	node.Active = true
	f.meta.KnownNodesRPCEndpoint[key] = node
	return nil
}

func (f *ClusterFSM) applyNodeDown(key string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	f.meta.KnownNodesRPCEndpoint[key].Active = false
	return nil
}

func (f *ClusterFSM) applyNodeLeave(key string) interface{} {
	f.l.Lock()
	defer f.l.Unlock()
	delete(f.meta.KnownNodesRPCEndpoint, key)
	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (f *ClusterFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.l.Lock()
	defer f.l.Unlock()
	return &fsmSnapshot{Metadata: util.DeepCopy(f.meta)}, nil
}

// Restore stores the key-value store to a previous state.
func (f *ClusterFSM) Restore(rc io.ReadCloser) error {
	o := Metadata{}
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	log.Info("raft restored: ", o)

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.meta = o
	return nil
}

type fsmSnapshot struct {
	Metadata interface{}
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.Metadata)
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

	log.Info("raft persisted")

	return nil
}

func (f *fsmSnapshot) Release() {
	log.Info("raft released ")
}
