// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package orm

// Aggregation is the interface that all specific aggregation types must implement.
// It serves as a marker to group different aggregation structs.
type Aggregation interface {
	// AddNested adds a sub-aggregation, allowing for fluent chaining.
	AddNested(name string, sub Aggregation) Aggregation
	// GetNested retrieves the map of nested sub-aggregations.
	GetNested() map[string]Aggregation
	// GetParams retrieves any additional parameters specific to the aggregation type.
	GetParams() map[string]interface{}
	// SetParams sets additional parameters specific to the aggregation type.
	SetParams(params map[string]interface{})
}

const (
	// Metric types
	MetricAvg         = "avg"
	MetricSum         = "sum"
	MetricMin         = "min"
	MetricMax         = "max"
	MetricCount       = "count"
	MetricPercentiles = "percentiles"
	MetricTopHits     = "top_hits"
	MetricCardinality = "cardinality"
	MetricMedian      = "median_absolute_deviation"
	// Bucket types
	MetricBucketTerms         = "terms"
	MetricBucketDateHistogram = "date_histogram"
	MetricBucketFilter        = "filter"
	MetricDateRange           = "date_range"
	// Pipeline types
	MetricPipelineDerivative = "derivative"
	MetricSumBucket          = "sum_bucket"
)

// baseAggregation provides common functionality for all aggregation types,
// especially for handling nested aggregations.
type baseAggregation struct {
	// NestedAggs holds any sub-aggregations.
	NestedAggs map[string]Aggregation `json:"-"`
	// Params can hold additional parameters specific to certain aggregation types.
	Params map[string]interface{} `json:"-"`
}

// AddNested adds a sub-aggregation to the base aggregation.
func (b *baseAggregation) AddNested(name string, sub Aggregation) Aggregation {
	if b.NestedAggs == nil {
		b.NestedAggs = make(map[string]Aggregation)
	}
	b.NestedAggs[name] = sub
	// This method needs to be called on the concrete type to return the correct type for chaining.
	// We'll see this implemented in the concrete types below.
	return nil
}

// GetNested returns the map of nested aggregations.
func (b *baseAggregation) GetNested() map[string]Aggregation {
	return b.NestedAggs
}

// GetParams returns the additional parameters of the aggregation.
func (b *baseAggregation) GetParams() map[string]interface{} {
	return b.Params
}

// SetParams sets additional parameters for the aggregation.
func (b *baseAggregation) SetParams(params map[string]interface{}) {
	b.Params = params
}

// TermsAggregation represents a "group by" or "bucketing" operation on a field.
type TermsAggregation struct {
	baseAggregation
	Field   string
	Include string
	Size    int
}

// AddNested provides a correctly typed chained call for TermsAggregation.
func (a *TermsAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}

// MetricAggregation represents a single-value metric calculation (avg, sum, etc.).
type MetricAggregation struct {
	baseAggregation        // Although metrics rarely have sub-aggs in ES, the model allows it.
	Type            string `mapstructure:"-"` // Type of metric: "avg", "sum", etc. Not part of the decoded structure.
	Field           string
}

// NewMetricAggregation creates a new MetricAggregation of the specified type and field.
func NewMetricAggregation(metricType, field string) *MetricAggregation {
	switch metricType {
	case MetricAvg, MetricSum, MetricMin, MetricMax, MetricCount, MetricCardinality, MetricMedian, MetricTopHits:
		// Valid metric types
	default:
		panic("invalid metric type: " + metricType)
	}
	return &MetricAggregation{
		Type:  metricType,
		Field: field,
	}
}

// PipelineAggregation represents a pipeline aggregation that processes the output of other aggregations.
type PipelineAggregation struct {
	baseAggregation
	Type        string `mapstructure:"-"` // Type of pipeline: "derivative", "sum_bucket", etc. Not part of the decoded structure.
	BucketsPath string
}

// NewPipelineAggregation creates a new PipelineAggregation of the specified type and buckets path.
func NewPipelineAggregation(pipelineType, bucketsPath string) *PipelineAggregation {
	switch pipelineType {
	case MetricSumBucket:
		// Valid pipeline types
	default:
		panic("invalid pipeline type: " + pipelineType)
	}
	return &PipelineAggregation{
		Type:        pipelineType,
		BucketsPath: bucketsPath,
	}
}

// AddNested provides a correctly typed chained call for MetricAggregation.
func (a *MetricAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}

// DateHistogramAggregation represents bucketing documents by a date/time interval.
type DateHistogramAggregation struct {
	baseAggregation
	Field         string
	Interval      string // A generic interval string like "1d", "1M", "1h".
	Format        string
	TimeZone      string
	IntervalField string // es-specific field name for backward compatibility
}

// AddNested provides a correctly typed chained call for DateHistogramAggregation.
func (a *DateHistogramAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}

// PercentilesAggregation represents the "percentiles" metric aggregation.
type PercentilesAggregation struct {
	baseAggregation
	Field    string
	Percents []float64
}

// AddNested provides a correctly typed chained call for PercentilesAggregation.
func (a *PercentilesAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}

// DerivativeAggregation represents the "derivative" pipeline aggregation.
type DerivativeAggregation struct {
	baseAggregation
	BucketsPath string `json:"buckets_path"`
}

// AddNested provides a correctly typed chained call for DerivativeAggregation.
func (a *DerivativeAggregation) AddNested(name string, sub Aggregation) Aggregation {
	panic("DerivativeAggregation does not support nested aggregations")
}

type FilterAggregation struct {
	baseAggregation
	// Query holds the filter criteria for this aggregation.
	Query map[string]interface{} `json:"query"`
}

// AddNested provides a correctly typed chained call for FilterAggregation.
func (a *FilterAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}

type DateRangeAggregation struct {
	baseAggregation
	Field    string        `json:"field"`
	TimeZone string        `json:"time_zone,omitempty"`
	Format   string        `json:"format,omitempty"`
	Ranges   []interface{} `json:"ranges"`
}

// AddNested provides a correctly typed chained call for DateRangeAggregation.
func (a *DateRangeAggregation) AddNested(name string, sub Aggregation) Aggregation {
	a.baseAggregation.AddNested(name, sub)
	return a
}
