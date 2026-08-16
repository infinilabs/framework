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
	previousOrder := sectionCallbackOrder
	previousConfigs := configCallbacks
	previousLatest := latestConfig
	sectionCallbacks = map[string][]func(pCfg, cCfg *Config){}
	sectionCallbackOrder = nil
	configCallbacks = nil
	latestConfig = map[string]*Config{}
	t.Cleanup(func() {
		sectionCallbacks = previousSections
		sectionCallbackOrder = previousOrder
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

func TestDispatchConfigChangeEventRunsSectionCallbacksInRegistrationOrder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "gateway.yml")
	content := []byte("flow:\n  - name: flow-1\nrouter:\n  - name: router-1\nentry:\n  - name: entry-1\n")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	previousSections := sectionCallbacks
	previousOrder := sectionCallbackOrder
	previousConfigs := configCallbacks
	previousLatest := latestConfig
	sectionCallbacks = map[string][]func(pCfg, cCfg *Config){}
	sectionCallbackOrder = nil
	configCallbacks = nil
	latestConfig = map[string]*Config{}
	t.Cleanup(func() {
		sectionCallbacks = previousSections
		sectionCallbackOrder = previousOrder
		configCallbacks = previousConfigs
		latestConfig = previousLatest
	})

	var order []string
	NotifyOnConfigSectionChange("flow", func(pCfg, cCfg *Config) {
		order = append(order, "flow")
	})
	NotifyOnConfigSectionChange("router", func(pCfg, cCfg *Config) {
		order = append(order, "router")
	})
	NotifyOnConfigSectionChange("entry", func(pCfg, cCfg *Config) {
		order = append(order, "entry")
	})

	dispatchConfigChangeEvent(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)

	expected := []string{"flow", "router", "entry"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d callbacks, got %d (%v)", len(expected), len(order), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Fatalf("expected callback order %v, got %v", expected, order)
		}
	}
}
