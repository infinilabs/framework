package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/kv"
)

// memKV is an in-memory KVStore so queue-config registration works in unit
// tests without a store module.
type memKV struct {
	data map[string][]byte
}

func (m *memKV) Open() error  { return nil }
func (m *memKV) Close() error { return nil }
func (m *memKV) GetValue(bucket string, key []byte) ([]byte, error) {
	if v, ok := m.data[string(key)]; ok {
		return append([]byte(nil), v...), nil
	}
	return nil, nil
}
func (m *memKV) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	return m.GetValue(bucket, key)
}
func (m *memKV) AddValueCompress(bucket string, key, value []byte) error {
	return m.AddValue(bucket, key, value)
}
func (m *memKV) AddValue(bucket string, key, value []byte) error {
	m.data[string(key)] = append([]byte(nil), value...)
	return nil
}
func (m *memKV) ExistsKey(bucket string, key []byte) (bool, error) {
	_, ok := m.data[string(key)]
	return ok, nil
}
func (m *memKV) DeleteKey(bucket string, key []byte) error {
	delete(m.data, string(key))
	return nil
}

func init() {
	kv.Register("test-mem-kv", &memKV{data: map[string][]byte{}})
}

func registerLabeledQueue(t *testing.T, id string, labels map[string]interface{}) *QueueConfig {
	t.Helper()
	cfg := AcquireQueueConfig()
	cfg.ID = id
	cfg.Name = id
	cfg.Labels = labels
	_, err := RegisterConfig(cfg)
	assert.NoError(t, err)
	t.Cleanup(func() { RemoveConfig(cfg) })
	return cfg
}

func labelSet(cfgs []*QueueConfig) map[string]bool {
	set := map[string]bool{}
	for _, c := range cfgs {
		set[c.ID] = true
	}
	return set
}

// A queue must carry ALL requested label pairs (AND): the historical
// behavior matched on any single pair, so a {topic:x} queue was selected
// by a {topic:x, env:prod} selector even without the env tag.
func TestGetConfigByLabels_AndSemantics(t *testing.T) {
	registerLabeledQueue(t, "labels-single", map[string]interface{}{"topic": "x"})
	registerLabeledQueue(t, "labels-other", map[string]interface{}{"topic": "y", "env": "prod"})
	registerLabeledQueue(t, "labels-extra", map[string]interface{}{"topic": "x", "env": "prod", "zone": "a"})

	// full subset match
	registerLabeledQueue(t, "labels-full", map[string]interface{}{"topic": "x", "env": "prod"})
	got := labelSet(GetConfigByLabels(map[string]interface{}{"topic": "x", "env": "prod"}))
	assert.True(t, got["labels-full"], "exact match must be selected")
	assert.True(t, got["labels-extra"], "queue with extra labels must be selected")
	assert.False(t, got["labels-single"], "queue missing a requested label must NOT be selected (AND)")
	assert.False(t, got["labels-other"], "queue with a different value must NOT be selected")

	// single-label selector still selects every queue carrying that pair
	got = labelSet(GetConfigByLabels(map[string]interface{}{"topic": "x"}))
	assert.True(t, got["labels-full"])
	assert.True(t, got["labels-single"])
	assert.True(t, got["labels-extra"])
	assert.False(t, got["labels-other"])

	// label values compare by string form (yaml int 1 vs "1")
	registerLabeledQueue(t, "labels-num", map[string]interface{}{"topic": "x", "shard": 3})
	got = labelSet(GetConfigByLabels(map[string]interface{}{"topic": "x", "shard": "3"}))
	assert.True(t, got["labels-num"], "label values must compare by string representation")

	// empty selector matches nothing
	assert.Empty(t, GetConfigByLabels(map[string]interface{}{}))

	// no-labels queues are never selected by a labeled selector
	registerLabeledQueue(t, "labels-none", nil)
	got = labelSet(GetConfigByLabels(map[string]interface{}{"topic": "x"}))
	assert.False(t, got["labels-none"])

	// selector with a label nobody carries
	assert.Empty(t, GetConfigByLabels(map[string]interface{}{"topic": "x", "missing": "1"}))
}

// The selector path consumers actually use must inherit the AND semantics.
func TestGetConfigBySelector_Labels(t *testing.T) {
	registerLabeledQueue(t, "sel-full", map[string]interface{}{"role": "logs", "env": "prod"})
	registerLabeledQueue(t, "sel-partial", map[string]interface{}{"role": "logs"})

	got := labelSet(GetConfigBySelector(&QueueSelector{Labels: map[string]interface{}{"role": "logs", "env": "prod"}}))
	assert.True(t, got["sel-full"])
	assert.False(t, got["sel-partial"])
}
