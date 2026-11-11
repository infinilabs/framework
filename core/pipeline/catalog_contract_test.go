/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

func TestCatalog_CarriesContractFields(t *testing.T) {
	// a registered-with-contract processor surfaces scope + record_contract
	RegisterProcessorPlugin("test_catalog_contract", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_catalog_contract", RecordContract{Scope: ScopeRecord, Namespace: "document", Granularity: GranularityBatchCapable}}, nil
	})
	RegisterProcessorContract("test_catalog_contract", RecordContract{
		Scope: ScopeRecord, Namespace: "document", Granularity: GranularityBatchCapable,
		RequiredFields: []string{"document.content"},
	})

	cat := GetProcessorCatalog()
	// category defaults to misc for un-categorized names; search everywhere
	var entry util.MapStr
	for _, group := range cat {
		if m, ok := group.(util.MapStr); ok {
			if e, ok := m["test_catalog_contract"].(util.MapStr); ok {
				entry = e
			}
		}
	}
	if entry == nil {
		t.Fatal("contract processor missing from catalog")
	}
	if entry["scope"] != "record" {
		t.Fatalf("scope = %v", entry["scope"])
	}
	rc, ok := entry["record_contract"].(util.MapStr)
	if !ok || rc["namespace"] != "document" || rc["granularity"] != "batch_capable" {
		t.Fatalf("record_contract = %v", entry["record_contract"])
	}

	// unconstrained processors stay unconstrained (no contract registered,
	// never constructed)
	RegisterProcessorPlugin("test_catalog_plain", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_catalog_plain", RecordContract{}}, nil
	})
	cat2 := GetProcessorCatalog()
	var plain util.MapStr
	for _, group := range cat2 {
		if m, ok := group.(util.MapStr); ok {
			if e, ok := m["test_catalog_plain"].(util.MapStr); ok {
				plain = e
			}
		}
	}
	if plain == nil || plain["scope"] != "unconstrained" {
		t.Fatalf("plain entry = %v", plain)
	}
}
