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
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestTermsAggregation(t *testing.T) {
	// Create a terms aggregation
	termsAgg := TermsAggregation{
		Field: "field1",
		Size: 10,
	}
	termsAgg.AddNested("count_values", NewMetricAggregation(MetricCount, "field2")).AddNested("max_value", NewMetricAggregation(MetricMax, "field2"))
	assert.Equal(t, 2, len(termsAgg.GetNested()), "Expected one nested aggregation")
	assert.Equal(t, "field1", termsAgg.Field, "Expected field to be 'field1'")
	assert.Equal(t, 10, termsAgg.Size, "Expected size to be 10")
}

func TestDateHistogramAggregation(t *testing.T) {
	// Create a date histogram aggregation
	dateHistAgg := DateHistogramAggregation{
		Field:    "date_field",
		Interval: "1d",
	}
	dateHistAgg.AddNested("avg_value", NewMetricAggregation(MetricAvg, "value_field"))
	assert.Equal(t, 1, len(dateHistAgg.GetNested()), "Expected one nested aggregation")
	assert.Equal(t, "1d", dateHistAgg.Interval, "Expected interval to be '1d'")
}

func TestAggregationWithParams(t *testing.T) {
	termsAgg := TermsAggregation{
		Field: "field1",
		Size: 10,
	}
	params := map[string]interface{}{
			"order": map[string]string{
				"_count": "desc",
			},
		}
	termsAgg.SetParams(params)
	assert.Equal(t, "desc", termsAgg.Params["order"].(map[string]string)["_count"], "Expected order param to be set")
}
