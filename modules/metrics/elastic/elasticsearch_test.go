package elastic

import (
	"testing"

	coreelastic "infini.sh/framework/core/elastic"
)

func TestShouldCollectMetricsSkipsConsoleCollectorInAgentMode(t *testing.T) {
	collector := &ElasticsearchMetric{}
	meta := &coreelastic.ElasticsearchMetadata{
		Config: &coreelastic.ElasticsearchConfig{
			Name:                 "agent-cluster",
			Enabled:              true,
			Monitored:            true,
			MetricCollectionMode: coreelastic.ModeAgent,
		},
	}

	if collector.shouldCollectMetrics(meta) {
		t.Fatal("expected console-side collector to skip clusters in agent mode")
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
}
