/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package env

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeTempConfig builds a reloadable config file: path.data/path.logs are
// always pinned to temp dirs — after the reload replaces SystemConfig, the
// defaults would otherwise point at relative "data"/"logs" inside the package
// directory. extra (may be empty) carries additional yaml sections.
func writeTempConfig(t *testing.T, extra string) string {
	t.Helper()
	cfg := "path:\n  data: " + t.TempDir() + "\n  logs: " + t.TempDir() + "\n" + extra
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setTestBindings(e *Env) {
	e.SystemConfig.WebAppConfig.NetworkConfig.SetEffectiveBinding("127.0.0.1:48101")
	e.SystemConfig.APIConfig.NetworkConfig.SetEffectiveBinding("127.0.0.1:48102")
}

func assertTestBindings(t *testing.T, e *Env) {
	t.Helper()
	if got := e.SystemConfig.WebAppConfig.NetworkConfig.EffectiveBinding; got != "127.0.0.1:48101" {
		t.Fatalf("web EffectiveBinding = %q, want 127.0.0.1:48101", got)
	}
	if got := e.SystemConfig.APIConfig.NetworkConfig.EffectiveBinding; got != "127.0.0.1:48102" {
		t.Fatalf("api EffectiveBinding = %q, want 127.0.0.1:48102", got)
	}
}

// A successful reload replaces SystemConfig wholesale but must keep the live
// listeners' runtime bindings.
func TestRefreshConfigPreservesRuntimeBindings(t *testing.T) {
	e := EmptyEnv()
	e.SetConfigFile(writeTempConfig(t, ""))
	setTestBindings(e)
	if err := e.RefreshConfig(); err != nil {
		t.Fatal(err)
	}
	assertTestBindings(t, e)
}

// A load that fails AFTER SystemConfig was replaced (configs-dir error with
// panic_on_config_error=false) must still restore the runtime bindings.
func TestRefreshConfigRestoresBindingsOnLateLoadError(t *testing.T) {
	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "broken.yml"), []byte("{{{ not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	extra := "  configs: " + badDir + "\nconfigs:\n  panic_on_config_error: false\n"
	e := EmptyEnv()
	e.SetConfigFile(writeTempConfig(t, extra))
	setTestBindings(e)
	if err := e.RefreshConfig(); err == nil {
		t.Fatal("expected the broken configs dir to fail the reload")
	}
	assertTestBindings(t, e)
}

// Concurrent reloads must not observe the replace/restore gap as empty
// bindings (run with -race for the synchronization guarantee).
func TestRefreshConfigConcurrentReloadsKeepBindings(t *testing.T) {
	e := EmptyEnv()
	e.SetConfigFile(writeTempConfig(t, ""))
	setTestBindings(e)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.RefreshConfig()
		}()
	}
	wg.Wait()
	assertTestBindings(t, e)
}
