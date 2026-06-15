package elastic

import (
	"testing"
	"time"

	"infini.sh/framework/core/orm"
)

func TestGetActiveHostPrefersAvailableSeedHostOverCachedDiscoveredHost(t *testing.T) {
	const (
		clusterID      = "docker-mapped-port-cluster"
		seedHost       = "192.168.3.185:9211"
		discoveredHost = "172.18.1.18:9200"
	)

	cfg := &ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: clusterID},
		Name:          clusterID,
		Host:          seedHost,
		Hosts:         []string{seedHost},
		Enabled:       true,
	}
	cfg.Discovery.Enabled = true

	meta := &ElasticsearchMetadata{
		Config: cfg,
		Nodes: &map[string]NodesInfo{
			"node-1": {
				Http: struct {
					BoundAddress            []string `json:"bound_address"`
					PublishAddress          string   `json:"publish_address,omitempty"`
					MaxContentLengthInBytes int64    `json:"max_content_length_in_bytes,omitempty"`
				}{
					PublishAddress: discoveredHost,
				},
			},
		},
		activeHost: &NodeAvailable{Host: discoveredHost, available: true, lastCheck: time.Now()},
	}

	hosts.Store(seedHost, &NodeAvailable{Host: seedHost, ClusterID: clusterID, available: true, lastCheck: time.Now()})
	hosts.Store(discoveredHost, &NodeAvailable{Host: discoveredHost, ClusterID: clusterID, available: true, lastCheck: time.Now()})
	t.Cleanup(func() {
		hosts.Delete(seedHost)
		hosts.Delete(discoveredHost)
	})

	got := meta.GetActiveHost()
	if got != seedHost {
		t.Fatalf("expected seed host %q to be preferred over discovered host %q, got %q", seedHost, discoveredHost, got)
	}
	if meta.activeHost == nil || meta.activeHost.Host != seedHost {
		t.Fatalf("expected activeHost to be updated to seed host %q, got %#v", seedHost, meta.activeHost)
	}
}

func TestGetActiveHostFallsBackToCachedDiscoveredHostWhenSeedUnavailable(t *testing.T) {
	const (
		clusterID      = "docker-discovery-fallback-cluster"
		seedHost       = "192.168.3.185:9211"
		discoveredHost = "172.18.1.18:9200"
	)

	cfg := &ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: clusterID},
		Name:          clusterID,
		Host:          seedHost,
		Hosts:         []string{seedHost},
		Enabled:       true,
	}
	cfg.Discovery.Enabled = true

	meta := &ElasticsearchMetadata{
		Config:     cfg,
		activeHost: &NodeAvailable{Host: discoveredHost, available: true, lastCheck: time.Now()},
	}

	hosts.Store(seedHost, &NodeAvailable{Host: seedHost, ClusterID: clusterID, available: false, lastCheck: time.Now()})
	hosts.Store(discoveredHost, &NodeAvailable{Host: discoveredHost, ClusterID: clusterID, available: true, lastCheck: time.Now()})
	t.Cleanup(func() {
		hosts.Delete(seedHost)
		hosts.Delete(discoveredHost)
	})

	got := meta.GetActiveHost()
	if got != discoveredHost {
		t.Fatalf("expected discovered host %q when seed host is unavailable, got %q", discoveredHost, got)
	}
}

func TestGetActiveHostInitializesAvailableSeedHostInfoFromAvailabilityCache(t *testing.T) {
	const (
		clusterID = "seed-host-cache-init-cluster"
		seedHost  = "192.168.3.185:9220"
	)

	cfg := &ElasticsearchConfig{
		ORMObjectBase: orm.ORMObjectBase{ID: clusterID},
		Name:          clusterID,
		Host:          seedHost,
		Hosts:         []string{seedHost},
		Enabled:       true,
	}

	meta := &ElasticsearchMetadata{Config: cfg}
	nodeAvailCache.Put(seedHost, true)
	hosts.Delete(seedHost)
	t.Cleanup(func() {
		hosts.Delete(seedHost)
	})

	got := meta.GetActiveHost()
	if got != seedHost {
		t.Fatalf("expected seed host %q, got %q", seedHost, got)
	}

	info, ok := GetHostAvailableInfo(seedHost)
	if !ok || info == nil {
		t.Fatalf("expected host info for %q to be initialized", seedHost)
	}
	if !info.IsAvailable() {
		t.Fatalf("expected host info for %q to be marked available", seedHost)
	}
	if info.ClusterID != clusterID {
		t.Fatalf("expected cluster id %q, got %q", clusterID, info.ClusterID)
	}
}

func TestShouldTraceUnavailableReasonForUnmonitoredCluster(t *testing.T) {
	meta := &ElasticsearchMetadata{
		Config: &ElasticsearchConfig{
			Monitored: false,
		},
	}

	if !meta.shouldTraceUnavailableReason() {
		t.Fatal("expected unmonitored cluster to trace unavailable reason")
	}

	meta.Config.Monitored = true
	if meta.shouldTraceUnavailableReason() {
		t.Fatal("expected monitored cluster to keep debug unavailable reason")
	}
}

func TestShouldCheckActiveHostsOnFailure(t *testing.T) {
	meta := &ElasticsearchMetadata{Config: &ElasticsearchConfig{}}
	if !meta.shouldCheckActiveHostsOnFailure() {
		t.Fatal("expected active hosts check enabled by default")
	}

	meta.Config.MetadataConfigs = &MetadataConfig{}
	meta.Config.MetadataConfigs.NodeAvailabilityCheck.Enabled = true
	if !meta.shouldCheckActiveHostsOnFailure() {
		t.Fatal("expected active hosts check enabled when node availability check is on")
	}

	meta.Config.MetadataConfigs.NodeAvailabilityCheck.Enabled = false
	if meta.shouldCheckActiveHostsOnFailure() {
		t.Fatal("expected active hosts check disabled when node availability check is off")
	}
}
