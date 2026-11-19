package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/util"
)

// EnsureNamedIDTypedConfig must guarantee ID == name: kafka_queue and other
// ID-keyed backends use the config ID as the physical topic, so an external
// topic queue must not fall back to the hashed dynamic ID.
func TestEnsureNamedIDTypedConfig(t *testing.T) {
	// fresh registration: ID stays the queue name, not the MD5 digest
	cfg := EnsureNamedIDTypedConfig("kafka", "lp-e2e")
	assert.Equal(t, "lp-e2e", cfg.ID)
	assert.Equal(t, "lp-e2e", cfg.Name)
	assert.Equal(t, "kafka", cfg.Type)
	byName, ok := GetConfigByKey("lp-e2e")
	assert.True(t, ok)
	assert.Equal(t, "lp-e2e", byName.ID)
	t.Cleanup(func() { RemoveConfig(cfg) })

	// idempotent: re-ensure keeps the named registration and flips the type
	cfg2 := EnsureNamedIDTypedConfig("disk", "lp-e2e")
	assert.Equal(t, "lp-e2e", cfg2.ID)
	assert.Equal(t, "disk", cfg2.Type)

	// a pre-existing dynamic registration (hashed ID) is replaced in place
	hashed := EnsureTypedConfig("kafka", "legacy-queue")
	assert.NotEqual(t, "legacy-queue", hashed.ID, "dynamic default should hash the name")
	fixed := EnsureNamedIDTypedConfig("kafka", "legacy-queue")
	assert.Equal(t, "legacy-queue", fixed.ID)
	if old, ok := GetConfigByUUID(hashed.ID); ok {
		t.Fatalf("hashed registration should have been removed, got %+v", old)
	}
	t.Cleanup(func() { RemoveConfig(fixed) })

	// file-declared queues already carry ID == name and are kept as-is
	fileCfg := &QueueConfig{ID: "from-file", Name: "from-file", Type: "disk", Source: "file"}
	_, err := RegisterConfig(fileCfg)
	assert.NoError(t, err)
	kept := EnsureNamedIDTypedConfig("kafka", "from-file")
	assert.Equal(t, "from-file", kept.ID)
	assert.Equal(t, "kafka", kept.Type)
	assert.Equal(t, "file", kept.Source)
	t.Cleanup(func() { RemoveConfig(fileCfg) })
}

// Replacing a hashed dynamic registration must carry over its
// identity-independent settings: label-based consumer selection
// (GetConfigByLabels) and codec config reference them, and must keep
// working under the new named ID.
func TestEnsureNamedIDTypedConfig_PreservesReplacedSettings(t *testing.T) {
	hashed := &QueueConfig{
		ID:     "legacy-hashed-id",
		Name:   "legacy-topic",
		Type:   "disk",
		Source: "dynamic",
		Codec:  "json",
		Labels: util.MapStr{"env": "test"},
	}
	_, err := RegisterConfig(hashed)
	assert.NoError(t, err)
	t.Cleanup(func() { RemoveConfig(hashed) })

	fixed := EnsureNamedIDTypedConfig("kafka", "legacy-topic")
	assert.Equal(t, "legacy-topic", fixed.ID)
	assert.Equal(t, "kafka", fixed.Type)
	assert.Equal(t, "json", fixed.Codec, "codec must survive the replacement")
	assert.Equal(t, util.MapStr{"env": "test"}, fixed.Labels, "labels must survive the replacement")
	t.Cleanup(func() { RemoveConfig(fixed) })

	matched := GetConfigByLabels(map[string]interface{}{"env": "test"})
	found := false
	for _, c := range matched {
		if c.ID == "legacy-topic" {
			found = true
		}
	}
	assert.True(t, found, "label-based selection must find the replaced queue")
}

// A hashed dynamic registration may live only in kv (registered on another
// node or before a restart, never loaded here): SmartGetConfig is
// memory-only and misses it, but GetConfigByUUID's kv fallback would
// resurrect it into the cache. The named-ID registration must clean the
// row up.
func TestEnsureNamedIDTypedConfig_CleansKVOnlyHashedRegistration(t *testing.T) {
	// register dynamically (ID = MD5digest(name), persisted to kv), then
	// drop it from memory only — leaving the kv row behind
	hashed := EnsureTypedConfig("kafka", "kv-only-topic")
	assert.NotEqual(t, "kv-only-topic", hashed.ID)
	digest := hashed.ID
	RemoveConfig(hashed)

	if _, ok := GetConfigByKey("kv-only-topic"); ok {
		t.Fatal("precondition: registration should be gone from memory")
	}

	fixed := EnsureNamedIDTypedConfig("kafka", "kv-only-topic")
	assert.Equal(t, "kv-only-topic", fixed.ID)
	t.Cleanup(func() { RemoveConfig(fixed) })

	if resurrected, ok := GetConfigByUUID(digest); ok {
		t.Fatalf("kv-only hashed registration must not be resurrectable, got %+v", resurrected)
	}
}
