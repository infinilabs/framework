package client

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"infini.sh/framework/core/global"
)

// The pipeline module is optional: with its providers unset the runtime
// snapshot must return nil, not panic the sync path (PR #433 review).
func TestRuntimeSnapshotWithoutPipelineProviders(t *testing.T) {
	oldSnap, oldFlags := getPipelineRuntimeSnapshot, getInstanceFlags
	getPipelineRuntimeSnapshot, getInstanceFlags = nil, nil
	defer func() { getPipelineRuntimeSnapshot, getInstanceFlags = oldSnap, oldFlags }()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("configs client without the pipeline module panicked: %v", r)
		}
	}()
	if got := runtimePipelineSnapshot(); got != nil {
		t.Fatalf("want nil report without the pipeline module, got %v", got)
	}
}

// A real managed sync (managed=true, valid config file) must survive
// without PipeModule.Setup — the first sync used to nil-panic inside
// runtimePipelineSnapshot (PR #433 review).
func TestManagedSyncWithoutPipelineSetup(t *testing.T) {
	if getPipelineRuntimeSnapshot != nil {
		t.Fatal("unexpected pipeline provider before setup")
	}
	cfg := global.Env().SystemConfig

	env := global.Env()
	oldFile, oldDir := env.GetConfigFile(), cfg.PathConfig.Config
	dir := t.TempDir()
	file := filepath.Join(dir, "app.yml")
	if err := os.WriteFile(file, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env.SetConfigFile(file)
	cfg.PathConfig.Config = dir
	defer func() { env.SetConfigFile(oldFile); cfg.PathConfig.Config = oldDir }()
	old := cfg.Configs.Managed
	cfg.Configs.Managed = true
	defer func() { cfg.Configs.Managed = old }()
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			if !strings.Contains(stack, "runtimePipelineSnapshot") {
				t.Fatalf("unrelated setup panic: %v\n%s", r, stack)
			}
			t.Fatalf("real managed sync without PipeModule.Setup panicked in runtimePipelineSnapshot: %v", r)
		}
	}()
	if err := ListenConfigChanges(); err != nil {
		t.Fatal(err)
	}
}
