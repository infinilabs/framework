package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplacer(t *testing.T) {
	assert.Equal(t, PrometheusMetricReplacer.Replace("invalid_metric-with-index_name"), "invalid_metric_with_index_name")
	assert.Equal(t, PrometheusMetricReplacer.Replace("invalid_metric-with-index.name"), "invalid_metric_with_index_name")
}
