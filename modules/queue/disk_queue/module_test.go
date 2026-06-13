package queue

import (
	"testing"

	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

func TestNormalizeDiskQueueConfigAppliesRobustWriteDefaults(t *testing.T) {
	cfg := &DiskQueueConfig{}

	normalizeDiskQueueConfig(cfg)

	if cfg.WriteTimeoutInMS != defaultWriteTimeoutInMS {
		t.Fatalf("unexpected write timeout: %d", cfg.WriteTimeoutInMS)
	}
	if cfg.WriteChanBuffer != defaultWriteChanBuffer {
		t.Fatalf("unexpected write chan buffer: %d", cfg.WriteChanBuffer)
	}
}

func TestNormalizeDiskQueueConfigKeepsExplicitWriteSettings(t *testing.T) {
	cfg := &DiskQueueConfig{
		WriteTimeoutInMS: 45 * 1000,
		WriteChanBuffer:  64,
	}

	normalizeDiskQueueConfig(cfg)

	if cfg.WriteTimeoutInMS != 45*1000 {
		t.Fatalf("write timeout should be preserved, got %d", cfg.WriteTimeoutInMS)
	}
	if cfg.WriteChanBuffer != 64 {
		t.Fatalf("write chan buffer should be preserved, got %d", cfg.WriteChanBuffer)
	}
}

func TestSetupDefaultsDeleteAfterCompress(t *testing.T) {
	env1 := EmptyEnv()
	global.RegisterEnv(env1)

	module := DiskQueue{}
	module.Setup()

	if !module.cfg.Compress.DeleteAfterCompress {
		t.Fatalf("delete_after_compress should default to true")
	}
}
