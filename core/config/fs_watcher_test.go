package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestDispatchConfigChangeEventRunsSectionCallbacksBeforeGenericCallbacks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "generated_metrics_tasks.yml")
	content := []byte("elasticsearch:\n  - id: \"cluster-1\"\n    name: \"cluster-1\"\n    enabled: true\n    endpoint: \"http://127.0.0.1:9200\"\n")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	previousSections := sectionCallbacks
	previousConfigs := configCallbacks
	previousLatest := latestConfig
	sectionCallbacks = map[string][]func(pCfg, cCfg *Config){}
	configCallbacks = nil
	latestConfig = map[string]*Config{}
	t.Cleanup(func() {
		sectionCallbacks = previousSections
		configCallbacks = previousConfigs
		latestConfig = previousLatest
	})

	var order []string
	NotifyOnConfigSectionChange("elasticsearch", func(pCfg, cCfg *Config) {
		order = append(order, "section")
	})
	NotifyOnConfigChange(func(ev fsnotify.Event) {
		order = append(order, "generic")
	})

	dispatchConfigChangeEvent(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)

	if len(order) != 2 {
		t.Fatalf("expected 2 callbacks, got %d (%v)", len(order), order)
	}
	if order[0] != "section" || order[1] != "generic" {
		t.Fatalf("expected section callback before generic callback, got %v", order)
	}
}
