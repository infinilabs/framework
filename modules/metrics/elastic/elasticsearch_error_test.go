package elastic

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetMetricTaskTimeoutUsesConfiguredInterval(t *testing.T) {
	if got := getMetricTaskTimeout("37s"); got != 37*time.Second {
		t.Fatalf("expected configured timeout, got %s", got)
	}
}

func TestGetMetricTaskTimeoutFallsBackToDefault(t *testing.T) {
	if got := getMetricTaskTimeout("invalid"); got != 10*time.Second {
		t.Fatalf("expected default timeout, got %s", got)
	}
}

func TestWrapMetricCollectErrorIncludesTimeoutContext(t *testing.T) {
	err := wrapMetricCollectError("cluster-a", "cluster_health", "http://127.0.0.1:9200", "15s", context.DeadlineExceeded)
	if !strings.Contains(err.Error(), "timed out after 15s") {
		t.Fatalf("expected timeout message, got %v", err)
	}
}

func TestWrapMetricPersistErrorIncludesMetricName(t *testing.T) {
	err := wrapMetricPersistError("cluster-a", "cluster_stats", context.DeadlineExceeded)
	if !strings.Contains(err.Error(), "persist cluster_stats") {
		t.Fatalf("expected persist message to include metric name, got %v", err)
	}
}
