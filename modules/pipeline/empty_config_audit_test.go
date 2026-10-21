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
	// register the processor catalog the shipping binaries carry
	// (direct imports: the plugins umbrella pulls optional deps like clashx)
	_ "infini.sh/framework/plugins/elastic/bulk_indexing"
	_ "infini.sh/framework/plugins/elastic/indexing_merge"
	_ "infini.sh/framework/plugins/elastic/json_indexing"
	_ "infini.sh/framework/plugins/elastic/lane_bulk"
	_ "infini.sh/framework/plugins/elastic/merge_to_bulk"
	_ "infini.sh/framework/plugins/enterprise/otlp/export"
	_ "infini.sh/framework/plugins/enterprise/processors/decode_duration"
	_ "infini.sh/framework/plugins/enterprise/processors/fingerprint"
	_ "infini.sh/framework/plugins/enterprise/processors/string"
	_ "infini.sh/framework/plugins/enterprise/processors/xml"
	_ "infini.sh/framework/plugins/enterprise/processors/dateformat"
	_ "infini.sh/framework/plugins/enterprise/processors/kv"
	_ "infini.sh/framework/plugins/enterprise/processors/redact"
	_ "infini.sh/framework/plugins/enterprise/processors/sample"
	_ "infini.sh/framework/plugins/enterprise/processors/web_log"
	_ "infini.sh/framework/plugins/enterprise/processors/cidr"
	_ "infini.sh/framework/plugins/enterprise/processors/communityid"
	_ "infini.sh/framework/plugins/enterprise/processors/drop_event"
	_ "infini.sh/framework/plugins/enterprise/processors/enrich_es"
	_ "infini.sh/framework/plugins/enterprise/processors/script"
	_ "infini.sh/framework/plugins/enterprise/processors/clone"
	_ "infini.sh/framework/plugins/enterprise/processors/dissect"
	_ "infini.sh/framework/plugins/enterprise/processors/mutate"
	_ "infini.sh/framework/plugins/enterprise/processors/registered_domain"
	_ "infini.sh/framework/plugins/enterprise/processors/urldecode"
	_ "infini.sh/framework/plugins/enterprise/processors/add_locale"
	_ "infini.sh/framework/plugins/enterprise/processors/drop_fields"
	_ "infini.sh/framework/plugins/enterprise/processors/field_standardize"
	_ "infini.sh/framework/plugins/enterprise/processors/geoip"
	_ "infini.sh/framework/plugins/enterprise/processors/syslogdecode"
	_ "infini.sh/framework/plugins/enterprise/processors/url"
	_ "infini.sh/framework/plugins/enterprise/processors/context_enrich"
	_ "infini.sh/framework/plugins/enterprise/processors/csv"
	_ "infini.sh/framework/plugins/enterprise/processors/date"
	_ "infini.sh/framework/plugins/enterprise/processors/decode_base64_field"
	_ "infini.sh/framework/plugins/enterprise/processors/decompress_gzip_field"
	_ "infini.sh/framework/plugins/enterprise/processors/extract_array"
	_ "infini.sh/framework/plugins/enterprise/processors/grok"
	_ "infini.sh/framework/plugins/enterprise/processors/json"
	_ "infini.sh/framework/plugins/enterprise/processors/otel_normalize"
	_ "infini.sh/framework/plugins/enterprise/processors/pattern_tagger"
	_ "infini.sh/framework/plugins/enterprise/processors/pizza_bulk"
	_ "infini.sh/framework/plugins/enterprise/processors/throttle"
	_ "infini.sh/framework/plugins/enterprise/processors/useragent"
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
	// audited: every parameter has a working default
	allowlist := map[string]bool{
		"add_locale":            true, // locale auto-detect
		"bulk_indexing":         true, // detect_active_queue auto-discovery mode
		"clone":                 true, // clone whole record
		"community_id":          true, // standard seed defaults
		"context_enrich":        true, // promote-all default fields
		"csv":                   true, // field defaults to otel message
		"date":                  true, // field defaults to otel timestamp
		"date_format":           true, // source defaults to record timestamp
		"decode_duration":       true, // field defaults to "duration"
		"decompress_gzip_field": true,
		"drop_event":            true, // unconditional drop
		"drop_fields":           true, // no-op until fields set — harmless
		"fingerprint":           true, // hashes the whole record
		"for_each":              true, // host — inner chain required, checked separately
		"json":                  true, // parse default field
		"kv":                    true, // field + defaults
		"merge_to_bulk":         true,
		"mutate":                true, // no-op until operations set — harmless
		"otel_normalize":        true, // normalization rules built-in
		"output":                true, // queue defaults to event routing
		"pizza_bulk":            true,
		"registered_domain":     true, // field defaults
		"replay":                true, // localhost:9200 test-utility default
		"sample":                true, // ratio defaults 0.5, range-checked
		"tap":                   true, // taps everything by default
		"throttle":              true,
		"url":                   true, // field defaults to uri // 100 events/1s/tag defaults, all validated
		"urldecode":             true, // field defaults
		"useragent":             true, // field defaults
		"web_log":               true, // parser defaults
		"field_standardize":     true, // mode default
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
