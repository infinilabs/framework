package elastic

import (
	"testing"
	"time"

	coreelastic "infini.sh/framework/core/elastic"
)

func TestShouldCollectMetricsAllowsConsoleCollectorInAgentModeForClusterLevelMetrics(t *testing.T) {
	collector := &ElasticsearchMetric{}
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			Name:                 "agent-cluster",
			Enabled:              true,
			Monitored:            true,
			MetricCollectionMode: coreelastic.ModeAgent,
		},
	}

	if !collector.shouldCollectMetrics(meta) {
		t.Fatal("expected console-side collector to keep cluster-level metrics in agent mode")
	}
	if collector.shouldCollectNodeAndIndexMetrics(meta) {
		t.Fatal("expected console-side collector to skip node/index metrics in agent mode")
	}
}

func TestShouldCollectMetricsAllowsConsoleCollectorInAgentlessMode(t *testing.T) {
	collector := &ElasticsearchMetric{}
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			Name:                 "agentless-cluster",
			Enabled:              true,
			Monitored:            true,
			MetricCollectionMode: coreelastic.ModeAgentless,
		},
	}

	if !collector.shouldCollectMetrics(meta) {
		t.Fatal("expected console-side collector to run for agentless clusters")
	}
	if !collector.shouldCollectNodeAndIndexMetrics(meta) {
		t.Fatal("expected console-side collector to run node/index metrics for agentless clusters")
	}
}

func TestShouldCollectMetricsSkipsAgentCollectorInAgentlessMode(t *testing.T) {
	collector := &ElasticsearchMetric{IsAgentMode: true}
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			Name:                 "agentless-cluster",
			Enabled:              true,
			Monitored:            true,
			MetricCollectionMode: coreelastic.ModeAgentless,
		},
	}

	if collector.shouldCollectMetrics(meta) {
		t.Fatal("expected agent-side collector to skip agentless clusters")
	}
}

func TestShouldCollectMetricsAllowsAgentCollectorInAgentMode(t *testing.T) {
	collector := &ElasticsearchMetric{IsAgentMode: true}
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			Name:                 "agent-cluster",
			Enabled:              true,
			Monitored:            true,
			MetricCollectionMode: coreelastic.ModeAgent,
		},
	}

	if !collector.shouldCollectMetrics(meta) {
		t.Fatal("expected agent-side collector to run for agent mode clusters")
	}
}

func TestGetMetricTaskInitialDelayStableAndBounded(t *testing.T) {
	first := getMetricTaskInitialDelay("cluster-a", "cluster_health", "10s")
	second := getMetricTaskInitialDelay("cluster-a", "cluster_health", "10s")
	if first != second {
		t.Fatalf("expected stable delay, got %s and %s", first, second)
	}

	delay, err := time.ParseDuration(first)
	if err != nil {
		t.Fatalf("expected parseable delay, got %q: %v", first, err)
	}
	if delay < 0 || delay >= 10*time.Second {
		t.Fatalf("expected delay to be within interval, got %s", delay)
	}
}

func TestGetMetricTaskInitialDelayVariesByTaskKind(t *testing.T) {
	healthDelay := getMetricTaskInitialDelay("cluster-a", "cluster_health", "10s")
	statsDelay := getMetricTaskInitialDelay("cluster-a", "cluster_stats", "10s")
	if healthDelay == statsDelay {
		t.Fatalf("expected different metric kinds to spread across interval, both got %s", healthDelay)
	}
}

func TestGetMetricTaskTimeoutFallsBackToDefault(t *testing.T) {
	if got := getMetricTaskTimeout("invalid"); got != 10*time.Second {
		t.Fatalf("expected default timeout, got %s", got)
	}
}
