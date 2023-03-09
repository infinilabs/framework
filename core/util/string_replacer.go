package util

import "strings"

// Prometheus metric name rule: [a-zA-Z_:][a-zA-Z0-9_:]*
// See https://prometheus.io/docs/concepts/data_model/
var PrometheusMetricReplacer = strings.NewReplacer(
	".", "_",
	"-", "_",
)
