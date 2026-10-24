/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"sync"
	"testing"

	"infini.sh/framework/core/pipeline"
)

// A processor name unique to this file — the plugin registry is
// process-global and Setup() (which registers "echo") never runs in tests,
// so a plain RegisterProcessorPlugin here cannot collide.
var hotReloadOnce sync.Once

func hotReloadTestProcessor() string {
	const name = "test_hotreload_echo"
	hotReloadOnce.Do(func() {
		pipeline.RegisterProcessorPlugin(name, NewEchoProcessor)
	})
	return name
}

func hotCfg(id, procName, message string) pipeline.PipelineConfigV2 {
	cfg := pipeline.PipelineConfigV2{
		Name:      id,
		AutoStart: true,
		Processors: []map[string]interface{}{
			{procName: map[string]interface{}{"message": message}},
		},
	}
	cfg.ID = id
	return cfg
}

// TestHotReload_SwapsRunningTask covers the ORM CRUD reload path (#131):
// a semantically identical edit is a no-op, a changed config drains and
// rebuilds the task with a generation bump, delete removes the task, and
// unknown IDs / transient tasks are left untouched.
func TestHotReload_SwapsRunningTask(t *testing.T) {
	proc := hotReloadTestProcessor()
	m := &PipeModule{}

	cfg1 := hotCfg("hr-1", proc, "v1")
	if err := m.createPipelineTask(cfg1, false); err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if _, ok := m.contexts.Load("hr-1"); !ok {
		t.Fatal("task context missing after create")
	}

	// semantically identical edit: no rebuild, no generation bump
	same := hotCfg("hr-1", proc, "v1")
	m.hotReloadPipeline("hr-1", &same)
	if got := m.generationOf("hr-1"); got != 1 {
		t.Fatalf("no-op edit bumped generation to %d", got)
	}

	// changed config: drain, rebuild, generation increments
	cfg2 := hotCfg("hr-1", proc, "v2")
	m.hotReloadPipeline("hr-1", &cfg2)
	stored, ok := m.configs.Load("hr-1")
	if !ok {
		t.Fatal("task config missing after hot reload")
	}
	if !stored.(pipeline.PipelineConfigV2).Equals(cfg2) {
		t.Fatalf("task not swapped to the new config: %+v", stored)
	}
	if _, ok := m.contexts.Load("hr-1"); !ok {
		t.Fatal("task context missing after hot reload")
	}
	if got := m.generationOf("hr-1"); got != 2 {
		t.Fatalf("generation = %d, want 2", got)
	}

	// the runtime report carries the generation so controllers can see
	// the reload landed
	snap := m.RuntimeSnapshot()
	entry, ok := snap["hr-1"].(map[string]interface{})
	if !ok {
		t.Fatalf("runtime snapshot has no entry for hr-1: %+v", snap)
	}
	if entry["generation"] != 2 {
		t.Fatalf("runtime snapshot generation = %v, want 2", entry["generation"])
	}

	// delete: stop and remove, no recreate
	m.hotReloadPipeline("hr-1", nil)
	if _, ok := m.configs.Load("hr-1"); ok {
		t.Fatal("config survives a delete hot reload")
	}
	if _, ok := m.contexts.Load("hr-1"); ok {
		t.Fatal("context survives a delete hot reload")
	}

	// unknown ID: no-op, no panic
	m.hotReloadPipeline("no-such-pipeline", nil)

	// transient tasks are never reloaded
	cfgT := hotCfg("hr-transient", proc, "t")
	if err := m.createPipelineTask(cfgT, true); err != nil {
		t.Fatalf("create transient: %v", err)
	}
	m.hotReloadPipeline("hr-transient", nil)
	if _, ok := m.configs.Load("hr-transient"); !ok {
		t.Fatal("transient task was reloaded")
	}
}
