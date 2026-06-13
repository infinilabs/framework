package adapter

import (
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

func TestGetClusterUUIDFallsBackToConfigWhenMetadataMissing(t *testing.T) {
	cfg := elastic.ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: "test-cluster-uuid-fallback"},
		Name:          "test-cluster-uuid-fallback",
		ClusterUUID:   "cluster-uuid-fallback",
	}

	t.Cleanup(func() {
		elastic.RemoveInstance(cfg.ID)
	})

	elastic.UpdateConfig(cfg)

	clusterUUID, err := GetClusterUUID(cfg.ID)
	if err != nil {
		t.Fatalf("expected cluster uuid from config fallback, got error: %v", err)
	}
	if clusterUUID != cfg.ClusterUUID {
		t.Fatalf("expected cluster uuid %q, got %q", cfg.ClusterUUID, clusterUUID)
	}
}
