/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

type noopProc struct{}

func (noopProc) Name() string             { return "test_domain_noop" }
func (noopProc) Process(c *Context) error { return nil }

func TestDomainRegistration(t *testing.T) {
	// register under a domain (bare name + qualified alias)
	RegisterDomainProcessor("testcat", "test_domain_noop", func(*config.Config) (Processor, error) {
		return noopProc{}, nil
	})

	t.Run("category recorded", func(t *testing.T) {
		if got := ProcessorCategory("test_domain_noop"); got != "testcat" {
			t.Fatalf("category = %q, want testcat", got)
		}
	})

	t.Run("default category is general", func(t *testing.T) {
		if got := ProcessorCategory("definitely-not-registered"); got != "general" {
			t.Fatalf("unknown processor category = %q, want general", got)
		}
	})

	t.Run("qualified alias resolvable", func(t *testing.T) {
		ctors := registry.ProcessorConstructors()
		if _, ok := ctors["testcat:test_domain_noop"]; !ok {
			t.Fatal("category-qualified alias not registered")
		}
	})

	t.Run("catalog groups by category", func(t *testing.T) {
		cat := GetProcessorCatalog()
		group, ok := cat["testcat"]
		if !ok {
			t.Fatalf("testcat group missing: %v", cat)
		}
		g, _ := group.(util.MapStr)
		if _, ok := g["test_domain_noop"]; !ok {
			t.Fatalf("processor missing from its category group: %v", group)
		}
		// qualified aliases must not appear as catalog entries
		if _, ok := g["testcat:test_domain_noop"]; ok {
			t.Fatal("qualified alias leaked into the catalog")
		}
	})

	t.Run("metadata carries category", func(t *testing.T) {
		meta := GetProcessorMetadata()
		m, ok := meta["test_domain_noop"].(util.MapStr)
		if !ok || m["category"] != "testcat" {
			t.Fatalf("flat metadata missing category: %v", meta["test_domain_noop"])
		}
	})
}

func TestBareNameCompatibility(t *testing.T) {
	// A processor registered the plain way has no category entry and
	// still shows up under "general" — pre-domain configs and callers
	// are unaffected.
	RegisterProcessorPlugin("test_plain_noop", func(*config.Config) (Processor, error) {
		return noopProc{}, nil
	})
	if got := ProcessorCategory("test_plain_noop"); got != "general" {
		t.Fatalf("plain registration category = %q, want general", got)
	}
	cat := GetProcessorCatalog()
	if _, ok := cat["general"]; !ok {
		t.Fatal("general group missing from catalog")
	}
}
