package common

import (
	"testing"

	"infini.sh/framework/core/elastic"
)

func TestGetInitialMetadataHealthDefaultsToAvailableForNewCluster(t *testing.T) {
	if !getInitialMetadataHealth(nil) {
		t.Fatal("expected new cluster metadata to start as available before first health check")
	}
}

func TestGetInitialMetadataHealthKeepsExistingAvailability(t *testing.T) {
	meta := &elastic.ElasticsearchMetadata{Config: &elastic.ElasticsearchConfig{Enabled: true}}
	meta.Init(false)

	if getInitialMetadataHealth(meta) {
		t.Fatal("expected existing unavailable metadata to remain unavailable")
	}

	meta.Init(true)
	if !getInitialMetadataHealth(meta) {
		t.Fatal("expected existing available metadata to remain available")
	}
}
