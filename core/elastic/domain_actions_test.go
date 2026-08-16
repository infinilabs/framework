package elastic

import (
	"testing"

	"infini.sh/framework/core/orm"
)

func TestRegisterInstanceInitializesMetadataOnFirstRegistration(t *testing.T) {
	cfg := ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: "test-first-sync"},
		Name:          "test-first-sync",
		Enabled:       true,
		ClusterUUID:   "cluster-uuid-1",
	}

	t.Cleanup(func() {
		cfgs.Delete(cfg.ID)
		apis.Delete(cfg.ID)
		metas.Delete(cfg.ID)
	})

	RegisterInstance(cfg, nil)

	meta := GetMetadata(cfg.ID)
	if meta == nil {
		t.Fatalf("expected metadata to be initialized for %s", cfg.ID)
	}
	if meta.Config == nil {
		t.Fatalf("expected metadata config to be initialized for %s", cfg.ID)
	}
	if meta.Config.ClusterUUID != cfg.ClusterUUID {
		t.Fatalf("expected cluster uuid %q, got %q", cfg.ClusterUUID, meta.Config.ClusterUUID)
	}
}
