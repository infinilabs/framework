package elastic

import (
	"testing"

	coreelastic "infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

func TestShouldRegisterDiscoveredHostForAvailabilityPrefersSeedHosts(t *testing.T) {
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			ORMObjectBase: orm.ORMObjectBase{ID: "cluster-1"},
			Host:          "192.168.3.8:9200",
			Hosts:         []string{"192.168.3.8:9200"},
		},
	}

	if shouldRegisterDiscoveredHostForAvailability(meta, "172.22.0.2:9200") {
		t.Fatal("expected non-seed discovered host to be excluded from availability monitoring")
	}

	if !shouldRegisterDiscoveredHostForAvailability(meta, "192.168.3.8:9200") {
		t.Fatal("expected seed host to remain eligible for availability monitoring")
	}
}

func TestShouldRegisterDiscoveredHostForAvailabilityAllowsDiscoveryWithoutSeeds(t *testing.T) {
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			ORMObjectBase: orm.ORMObjectBase{ID: "cluster-2"},
		},
	}

	if !shouldRegisterDiscoveredHostForAvailability(meta, "172.22.0.2:9200") {
		t.Fatal("expected discovered host to be eligible when no seed hosts are configured")
	}
}
