package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
