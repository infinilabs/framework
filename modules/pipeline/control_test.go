/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/pipeline"
)

// ── generation guard (#120) ─────────────────────────────────────────────

func TestRequestedGeneration(t *testing.T) {
	req := httptest.NewRequest("POST", "/pipeline/task/x/_pause?generation=2", nil)
	if n, ok := requestedGeneration(req); !ok || n != 2 {
		t.Fatalf("generation=2 parsed as (%d, %v)", n, ok)
	}
	req = httptest.NewRequest("POST", "/pipeline/task/x/_pause", nil)
	if _, ok := requestedGeneration(req); ok {
		t.Fatal("absent param must be unguarded")
	}
	req = httptest.NewRequest("POST", "/pipeline/task/x/_pause?generation=abc", nil)
	if _, ok := requestedGeneration(req); ok {
		t.Fatal("malformed param must be unguarded")
	}
	req = httptest.NewRequest("POST", "/pipeline/task/x/_pause?generation=0", nil)
	if _, ok := requestedGeneration(req); ok {
		t.Fatal("non-positive param must be unguarded")
	}
}

// ── drain (#121) ────────────────────────────────────────────────────────

func drainableCfg(id, procName string) pipeline.PipelineConfigV2 {
	cfg := hotCfg(id, procName, "x")
	cfg.KeepRunning = true
	cfg.RetryDelayInMs = 1
	return cfg
}

func TestDrainTask_ParksStoppedAndResumes(t *testing.T) {
	proc := hotReloadTestProcessor()
	m := &PipeModule{}
	defer func() {
		m.stopAndWaitForRelease([]string{"drain-1"}, time.Minute)
		m.deleteTask("drain-1")
	}()

	if err := m.createPipelineTask(drainableCfg("drain-1", proc), false); err != nil {
		t.Fatalf("create: %v", err)
	}

	if !m.drainTask("drain-1") {
		t.Fatal("drainTask reported task missing")
	}
	state := m.waitTaskSettled("drain-1", 5*time.Second)
	if state == pipeline.STARTED || state == pipeline.STARTING || state == pipeline.STOPPING {
		t.Fatalf("task not settled after drain, state=%v", state)
	}
	// drained task stays registered (resumable), unlike delete
	if _, ok := m.contexts.Load("drain-1"); !ok {
		t.Fatal("drained task was removed from the registry")
	}

	// restart clears the drain flag so the task runs again
	if !m.startTask("drain-1") {
		t.Fatal("startTask on drained task failed")
	}
	deadline := time.Now().Add(3 * time.Second)
	for m.taskRunningState("drain-1") == pipeline.STOPPED {
		if time.Now().After(deadline) {
			t.Fatal("task did not resume after start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	v, _ := m.contexts.Load("drain-1")
	if ctx, ok := v.(*pipeline.Context); !ok || ctx.IsDraining() {
		t.Fatal("restart did not clear the drain flag")
	}
}

func TestDrainAllTasks_SettlesEveryTask(t *testing.T) {
	proc := hotReloadTestProcessor()
	m := &PipeModule{}
	for _, id := range []string{"drainall-1", "drainall-2"} {
		if err := m.createPipelineTask(drainableCfg(id, proc), false); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		defer func(id string) {
			m.stopAndWaitForRelease([]string{id}, time.Minute)
			m.deleteTask(id)
		}(id)
	}

	states := m.drainAllTasks(5 * time.Second)
	if len(states) != 2 {
		t.Fatalf("drainAllTasks saw %d tasks, want 2: %v", len(states), states)
	}
	for id, s := range states {
		if s == string(pipeline.STARTED) || s == string(pipeline.STARTING) || s == string(pipeline.STOPPING) {
			t.Fatalf("task %s not settled: %v", id, s)
		}
	}
}

// ── cordon (#121) ───────────────────────────────────────────────────────

func TestCordon_BlocksTaskStarts(t *testing.T) {
	proc := hotReloadTestProcessor()
	m := &PipeModule{}
	defer func() {
		m.stopAndWaitForRelease([]string{"cordon-1"}, time.Minute)
		m.deleteTask("cordon-1")
	}()

	m.cordoned.Store(true)
	if err := m.createPipelineTask(drainableCfg("cordon-1", proc), false); err != nil {
		t.Fatalf("create under cordon: %v", err)
	}
	// auto_start task on a cordoned instance parks STOPPED instead of running
	deadline := time.Now().Add(3 * time.Second)
	for {
		s := m.taskRunningState("cordon-1")
		if s == pipeline.STOPPED {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cordoned task did not park, state=%v", s)
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if s := m.taskRunningState("cordon-1"); s != pipeline.STOPPED {
		t.Fatalf("cordoned task left the parked state: %v", s)
	}

	m.cordoned.Store(false)
	if !m.startTask("cordon-1") {
		t.Fatal("startTask after uncordon failed")
	}
	deadline = time.Now().Add(3 * time.Second)
	for m.taskRunningState("cordon-1") == pipeline.STOPPED {
		if time.Now().After(deadline) {
			t.Fatal("task did not resume after uncordon")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ── lock takeover (#122) ────────────────────────────────────────────────

// controlMemKV is the minimal in-memory KVStore backing the locker probes.
type controlMemKV struct {
	mu sync.Mutex
	kv map[string][]byte
}

func (m *controlMemKV) Open() error  { return nil }
func (m *controlMemKV) Close() error { return nil }

func (m *controlMemKV) key(bucket string, key []byte) string {
	return bucket + ":" + string(key)
}

func (m *controlMemKV) GetValue(bucket string, key []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.kv[m.key(bucket, key)]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (m *controlMemKV) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	return m.GetValue(bucket, key)
}

func (m *controlMemKV) AddValue(bucket string, key []byte, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kv[m.key(bucket, key)] = value
	return nil
}

func (m *controlMemKV) AddValueCompress(bucket string, key []byte, value []byte) error {
	return m.AddValue(bucket, key, value)
}

func (m *controlMemKV) ExistsKey(bucket string, key []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.kv[m.key(bucket, key)]
	return ok, nil
}

func (m *controlMemKV) DeleteKey(bucket string, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.kv, m.key(bucket, key))
	return nil
}

var controlKVOnce sync.Once

// placeLockRow writes a raw locker row ("clientID/<unix>") under the
// locker's private kv bucket; the bucket literal mirrors core/locker's
// parentBucket.
func placeLockRow(t *testing.T, name, clientID string, ts time.Time) {
	v := clientID + "/" + strconv.FormatInt(ts.Unix(), 10)
	if err := kv.AddValue("dis_locker", locker.GetKey(pipelineSingleton, name), []byte(v)); err != nil {
		t.Fatalf("place lock row: %v", err)
	}
}

func TestProbeLockTakeover(t *testing.T) {
	controlKVOnce.Do(func() {
		kv.Register("control-test-mem", &controlMemKV{kv: map[string][]byte{}})
	})

	const name = "takeover-probe-1"
	ttl := 60 * time.Second
	expired := time.Now().Add(-2 * ttl)

	if got := probeLockTakeover(pipelineSingleton, name, ttl); got != "" {
		t.Fatalf("no lock placed, probe returned %q", got)
	}

	placeLockRow(t, name, "node-A", expired)
	if got := probeLockTakeover(pipelineSingleton, name, ttl); got != "node-A" {
		t.Fatalf("expired foreign lock: probe returned %q, want node-A", got)
	}

	placeLockRow(t, name, "node-B", time.Now())
	if got := probeLockTakeover(pipelineSingleton, name, ttl); got != "" {
		t.Fatalf("fresh foreign lock must not report takeover, got %q", got)
	}

	placeLockRow(t, name, global.Env().SystemConfig.NodeConfig.ID, expired)
	if got := probeLockTakeover(pipelineSingleton, name, ttl); got != "" {
		t.Fatalf("self-owned lock must not report takeover, got %q", got)
	}
}

func TestRecordPipelineEvent_TakeoverLandsInFlight(t *testing.T) {
	installFlight()
	recordPipelineEvent("evt-takeover-1", pipeline.OutcomeTakeover, "singleton lock taken over from instance node-A")
	evs := flightQuery("evt-takeover-1", "", true, 10)
	if len(evs) != 1 {
		t.Fatalf("takeover event not retained: %+v", evs)
	}
	if evs[0].Outcome != pipeline.OutcomeTakeover {
		t.Fatalf("outcome = %v, want %v", evs[0].Outcome, pipeline.OutcomeTakeover)
	}
}
