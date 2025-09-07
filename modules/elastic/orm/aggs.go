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

import (
	"fmt"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

type ESAggregation struct {
	Terms         *esTermsAggregation         `json:"terms,omitempty"`
	DateHistogram *esDateHistogramAggregation `json:"date_histogram,omitempty"`
	Avg           *esMetricAggregation        `json:"avg,omitempty"`
	Sum           *esMetricAggregation        `json:"sum,omitempty"`
	Min           *esMetricAggregation        `json:"min,omitempty"`
	Max           *esMetricAggregation        `json:"max,omitempty"`
	Cardinality   *esMetricAggregation        `json:"cardinality,omitempty"`
	Percentiles   *esPercentilesAggregation   `json:"percentiles,omitempty"`
	NestedAggs    map[string]*ESAggregation   `json:"aggs,omitempty"`
	Count         *esMetricAggregation        `json:"value_count,omitempty"`
	Median        *esMetricAggregation        `json:"median_absolute_deviation,omitempty"`
	Derivative    *esPipelineAggregation      `json:"derivative,omitempty"`
	Filter        *esFilterAggregation        `json:"filter,omitempty"`
	TopHits       map[string]interface{}      `json:"top_hits,omitempty"`
	SumBucket     *esPipelineAggregation      `json:"sum_bucket,omitempty"`
	DateRange     *esDateRangeAggregation     `json:"date_range,omitempty"`
}

type esTermsAggregation struct {
	Field string `json:"field,omitempty"`
	Size  int    `json:"size,omitempty"`
}

type esMetricAggregation struct {
	Field string `json:"field,omitempty"`
}

type esPercentilesAggregation struct {
	Field    string    `json:"field,omitempty"`
	Percents []float64 `json:"percents,omitempty"`
}

type esDateHistogramAggregation struct {
	Field            string `json:"field,omitempty"`
	CalendarInterval string `json:"calendar_interval,omitempty"` // Note the ES-specific field name
	FixedInterval    string `json:"fixed_interval,omitempty"`    // Note the ES-specific field name
	Interval         string `json:"interval,omitempty"`          // Deprecated but still supported by ES
	Format           string `json:"format,omitempty"`
	TimeZone         string `json:"time_zone,omitempty"`
}

type esPipelineAggregation struct {
	BucketsPath string `json:"buckets_path,omitempty"`
}
type esFilterAggregation map[string]interface{}
type esDateRangeAggregation struct {
	Field    string        `json:"field,omitempty"`
	Format   string        `json:"format,omitempty"`
	Ranges   []interface{} `json:"ranges,omitempty"`
	TimeZone string        `json:"time_zone,omitempty"`
}

// AggreationBuilder is responsible for compiling an abstract aggreation Request into an ES query.
type AggreationBuilder struct{}

// NewAggreationBuilder creates a new Elasticsearch aggreation builder.
func NewAggreationBuilder() *AggreationBuilder {
	return &AggreationBuilder{}
}

// Build takes an abstract request and returns a JSON byte slice ready to be sent to Elasticsearch.
func (c *AggreationBuilder) Build(aggs map[string]orm.Aggregation) (any, error) {
	translatedAggs := make(map[string]*ESAggregation)

	// If there are aggregations, translate them.
	if len(aggs) > 0 {
		for name, agg := range aggs {
			esAgg, err := c.translateAggregation(agg)
			if err != nil {
				return nil, fmt.Errorf("failed to translate aggregation '%s': %w", name, err)
			}
			translatedAggs[name] = esAgg
		}
	}

	return translatedAggs, nil
}

// translateAggregation is a recursive function that converts an abstract Aggregation
// into its specific esAggregation representation.
func (c *AggreationBuilder) translateAggregation(agg orm.Aggregation) (*ESAggregation, error) {
	esAgg := &ESAggregation{}

	// Use a type switch to handle different kinds of abstract aggregations.
	switch v := agg.(type) {
	case *orm.TermsAggregation:
		esAgg.Terms = &esTermsAggregation{
			Field: v.Field,
			Size:  v.Size,
		}
	case *orm.MetricAggregation:
		metric := &esMetricAggregation{Field: v.Field}
		switch v.Type {
		case orm.MetricAvg:
			esAgg.Avg = metric
		case orm.MetricSum:
			esAgg.Sum = metric
		case orm.MetricMin:
			esAgg.Min = metric
		case orm.MetricMax:
			esAgg.Max = metric
		case orm.MetricCardinality:
			esAgg.Cardinality = metric
		case orm.MetricCount:
			esAgg.Count = metric
		case orm.MetricMedian:
			esAgg.Median = metric
		case orm.MetricTopHits:
			esAgg.TopHits = v.GetParams()
		default:
			return nil, fmt.Errorf("unsupported metric aggregation type: %s", v.Type)
		}
	case *orm.PercentilesAggregation:
		esAgg.Percentiles = &esPercentilesAggregation{
			Field:    v.Field,
			Percents: v.Percents,
		}
	case *orm.DateHistogramAggregation:

		esAgg.DateHistogram = &esDateHistogramAggregation{
			Field:    v.Field,
			Format:   v.Format,
			TimeZone: v.TimeZone,
		}
		switch v.IntervalField {
		case elastic.CalendarInterval:
			esAgg.DateHistogram.CalendarInterval = v.Interval
		case elastic.FixedInterval:
			esAgg.DateHistogram.FixedInterval = v.Interval
		default:
			esAgg.DateHistogram.Interval = v.Interval
		}
	case *orm.DerivativeAggregation:
		esAgg.Derivative = &esPipelineAggregation{
			BucketsPath: v.BucketsPath,
		}
	case *orm.FilterAggregation:
		esAgg.Filter = (*esFilterAggregation)(&v.Query)
	case *orm.PipelineAggregation:
		esAgg.SumBucket = &esPipelineAggregation{
			BucketsPath: v.BucketsPath,
		}
	case *orm.DateRangeAggregation:
		esAgg.DateRange = &esDateRangeAggregation{
			Field:    v.Field,
			Format:   v.Format,
			Ranges:   v.Ranges,
			TimeZone: v.TimeZone,
		}
	default:
		return nil, fmt.Errorf("unsupported aggregation type: %T", v)
	}

	// Recursively translate any nested aggregations.
	nested := agg.GetNested()
	if len(nested) > 0 {
		esAgg.NestedAggs = make(map[string]*ESAggregation)
		for name, subAgg := range nested {
			translatedSub, err := c.translateAggregation(subAgg)
			if err != nil {
				return nil, err // Propagate error
			}
			esAgg.NestedAggs[name] = translatedSub
		}
	}

	return esAgg, nil
}
