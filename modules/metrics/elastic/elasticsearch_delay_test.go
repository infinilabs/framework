package elastic

import (
	"testing"
	"time"
)

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
		t.Fatalf("expected different task kinds to spread to different offsets, got %s", healthDelay)
	}
}
