package elastic

import (
	"testing"

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
