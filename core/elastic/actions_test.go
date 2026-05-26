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
