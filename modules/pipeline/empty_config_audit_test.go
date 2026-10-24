/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline_test

import (
	"sort"
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
	// register the OSS processor catalog this repo ships (direct imports:
	// the plugins umbrella pulls optional deps like clashx). Enterprise
	// processors live in the separate enterprise-plugins overlay repo and
	// are audited by the twin test in that repo — importing them here would
	// break CI checkouts where the overlay is absent.
	_ "infini.sh/framework/plugins/elastic/bulk_indexing"
	_ "infini.sh/framework/plugins/elastic/indexing_merge"
	_ "infini.sh/framework/plugins/elastic/json_indexing"
	_ "infini.sh/framework/plugins/elastic/lane_bulk"
	_ "infini.sh/framework/plugins/elastic/merge_to_bulk"
	_ "infini.sh/framework/plugins/http"
	_ "infini.sh/framework/plugins/queue/consumer"
	_ "infini.sh/framework/plugins/queue/queue_output"
	_ "infini.sh/framework/plugins/replay"
	_ "infini.sh/framework/plugins/smtp"
)

// TestEmptyConfigAudit reports which processors accept an EMPTY config.
// Empty-accept is legitimate for processors whose every parameter has a
// sane default (drop_event, clone...), but a gap for processors that
// cannot work without a required input (dissect needs pattern...).
// The allowlist below documents the audited verdict per processor; a new
// processor that lands here must either gain required-field validation
// or be consciously added to the allowlist.
func TestEmptyConfigAudit(t *testing.T) {
	// audited: every parameter has a working default (OSS catalog only —
	// enterprise verdicts live in the twin test in the enterprise repo)
	allowlist := map[string]bool{
		"bulk_indexing": true, // detect_active_queue auto-discovery mode
		"for_each":      true, // host — inner chain required, checked separately
		"merge_to_bulk": true,
		"output":        true, // queue defaults to event routing
		"replay":        true, // localhost:9200 test-utility default
		"tap":           true, // taps everything by default
	}

	ctors := pipeline.ProcessorConstructors()
	names := make([]string, 0, len(ctors))
	for name := range ctors {
		names = append(names, name)
	}
	sort.Strings(names)

	accepted := []string{}
	panicked := []string{}
	for _, name := range names {
		ctor := ctors[name]
		cfg, err := config.NewConfigFrom(map[string]interface{}{})
		if err != nil {
			t.Fatalf("build empty config: %v", err)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = append(panicked, name)
				}
			}()
			if _, err := ctor(cfg); err == nil {
				accepted = append(accepted, name)
			}
		}()
	}
	if len(panicked) > 0 {
		t.Fatalf("processors PANIC on empty config (must return an error instead): %v", panicked)
	}

	// namespace aliases (parsing:csv == csv) share constructors with the
	// bare names — normalize before the allowlist lookup; test doubles
	// registered by sibling test files are out of catalog scope
	bare := func(name string) string {
		if i := strings.Index(name, ":"); i >= 0 {
			return name[i+1:]
		}
		return name
	}
	gaps := []string{}
	for _, name := range accepted {
		if strings.HasPrefix(name, "test_") {
			continue
		}
		if !allowlist[bare(name)] {
			gaps = append(gaps, name)
		}
	}
	if len(gaps) > 0 {
		t.Fatalf("processors accepting empty config without an audited default (add required-field validation or allowlist): %v", gaps)
	}
	t.Logf("empty-accept: %d processors, all audited", len(accepted))
}
