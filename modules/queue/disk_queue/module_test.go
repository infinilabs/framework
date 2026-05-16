package queue

import (
	"testing"

	. "infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

func TestSetupDefaultsDeleteAfterCompress(t *testing.T) {
	env1 := EmptyEnv()
	global.RegisterEnv(env1)

	module := DiskQueue{}
	module.Setup()

	if !module.cfg.Compress.DeleteAfterCompress {
		t.Fatalf("delete_after_compress should default to true")
	}
}
