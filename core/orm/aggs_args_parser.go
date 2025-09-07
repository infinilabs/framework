package orm

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// keyPartRegex is used to parse keys like "agg[key1][key2]" into parts: "agg", "key1", "key2".
var keyPartRegex = regexp.MustCompile(`^([^\[\]]+)|\[([^\[\]]*)\]`)

// ParseAggregationsFromQuery takes URL query values and converts them into a map of abstract aggregations.
func ParseAggregationsFromQuery(values url.Values) (map[string]Aggregation, error) {
	// Step 1: Convert flat URL params into a nested map.
	nestedMap, err := parseToNestedMap(values)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL values to nested map: %w", err)
	}

	// Step 2: Convert the nested map into our abstract aggregator structs.
	return buildAggregationsFromMap(nestedMap)
}

// parseToNestedMap converts flat url.Values with bracket notation into a nested map[string]interface{}.
func parseToNestedMap(values url.Values) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, val := range values {
		// We only care about keys starting with "agg".
		if !strings.HasPrefix(key, "agg") {
			continue
		}

		// Extract all parts from the key, e.g., "agg[types][terms][field]" -> ["agg", "types", "terms", "field"]
		matches := keyPartRegex.FindAllStringSubmatch(key, -1)
		var parts []string
		for _, match := range matches {
			var part string
			if match[1] != "" {
				part = match[1]
			} else {
				part = match[2]
			}

			decodedPart, err := url.QueryUnescape(part)
			if err != nil {
				// Fallback to the raw part if decoding fails
				decodedPart = part
			}
			parts = append(parts, decodedPart)
		}

		if len(parts) == 0 || parts[0] != "agg" {
			continue // Malformed key, skip.
		}

		// Navigate/create the nested map structure.
		currentMap := result
		for i, part := range parts[1:] { // Skip the "agg" prefix
			// If it's the last part, assign the value.
			if i == len(parts)-2 {
				currentMap[part] = val[0] // Assume single value per key.
				break
			}

			// If the next level map doesn't exist, create it.
			if _, ok := currentMap[part]; !ok {
				currentMap[part] = make(map[string]interface{})
			}

			// Move to the next level.
			nextMap, ok := currentMap[part].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("structure conflict in key '%s' at part '%s'", key, part)
			}
			currentMap = nextMap
		}
	}

	return result, nil
}

// stringToNumberHook is a mapstructure.DecodeHookFunc that converts string representations
// of numbers into actual numeric types (int, float64, etc.).
func stringToNumberHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Special handling for comma-separated strings to []float64 for percentiles
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Float64 {
			str := data.(string)
			parts := strings.Split(str, ",")
			floats := make([]float64, 0, len(parts))
			for _, part := range parts {
				f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
				if err != nil {
					// If any part is not a float, let mapstructure handle it
					return data, nil
				}
				floats = append(floats, f)
			}
			return floats, nil
		}

		// The target type must be a numeric type.
		isNumeric := t.Kind() >= reflect.Int && t.Kind() <= reflect.Float64
		if !isNumeric {
			return data, nil
		}

		str := data.(string)

		// Try to parse as integer first.
		if t.Kind() >= reflect.Int && t.Kind() <= reflect.Uint64 {
			n, err := strconv.ParseInt(str, 10, 64)
			if err == nil {
				return n, nil
			}
		}

		// Then try to parse as float.
		if t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64 {
			f, err := strconv.ParseFloat(str, 64)
			if err == nil {
				return f, nil
			}
		}

		// Return original data if parsing fails. Mapstructure will then report the error.
		return data, nil
	}
}

// decodeWithHooks is a helper function to decode a map into a struct using our custom hooks.
func decodeWithHooks(input map[string]interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		// Add our custom hook function here.
		DecodeHook: stringToNumberHook(),
		Result:     result,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(input)
}

// buildAggregationsFromMap is a recursive function that converts a map into Aggregation structs.
func buildAggregationsFromMap(aggMap map[string]interface{}) (map[string]Aggregation, error) {
	result := make(map[string]Aggregation)

	for name, data := range aggMap {
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("aggregation '%s' has invalid format", name)
		}

		var agg Aggregation
		var err error

		// Aggregation Factory: determine the type and create the struct.
		if termsData, ok := dataMap["terms"]; ok {
			var termsAgg TermsAggregation
			termsMap, ok := termsData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid format for terms aggregation '%s'", name)
			}
			if err = decodeWithHooks(termsMap, &termsAgg); err != nil {
				return nil, fmt.Errorf("failed to decode terms aggregation '%s': %w", name, err)
			}
			agg = &termsAgg
		} else if dateHistData, ok := dataMap["date_histogram"]; ok {
			var dateHistAgg DateHistogramAggregation
			dateHistMap, ok := dateHistData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid format for date_histogram aggregation '%s'", name)
			}
			if err = decodeWithHooks(dateHistMap, &dateHistAgg); err != nil {
				return nil, fmt.Errorf("failed to decode date_histogram '%s': %w", name, err)
			}
			agg = &dateHistAgg
		} else if percentilesData, ok := dataMap["percentiles"]; ok {
			var percentilesAgg PercentilesAggregation
			percentilesMap, ok := percentilesData.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid format for percentiles aggregation '%s'", name)
			}
			if err = decodeWithHooks(percentilesMap, &percentilesAgg); err != nil {
				return nil, fmt.Errorf("failed to decode percentiles aggregation '%s': %w", name, err)
			}
			agg = &percentilesAgg
		} else {
			// Handle various metric aggregations
			var metricAgg *MetricAggregation
			var metricType string

			for key, metricData := range dataMap {
				switch key {
				case "avg", "sum", "min", "max", "cardinality":
					if metricType != "" {
						return nil, fmt.Errorf("aggregation '%s' has multiple metric types", name)
					}
					metricType = key
					var m MetricAggregation
					metricMap, ok := metricData.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("invalid format for %s aggregation '%s'", key, name)
					}
					if err = decodeWithHooks(metricMap, &m); err != nil {
						return nil, fmt.Errorf("failed to decode %s aggregation '%s': %w", key, name, err)
					}
					m.Type = key
					metricAgg = &m
				case "aggs":
					// This will be handled later, after the metric agg is created.
				default:
					// Potentially unknown aggregation type in the same level as a metric.
					// Depending on strictness, could be an error. For now, we ignore.
				}
			}

			if metricAgg != nil {
				agg = metricAgg
			} else {
				// Skip if no known aggregation type is found. Could also be an error.
				continue
			}
		}

		// Recursively build nested aggregations.
		if nestedData, ok := dataMap["aggs"].(map[string]interface{}); ok {
			nestedAggs, err := buildAggregationsFromMap(nestedData)
			if err != nil {
				return nil, fmt.Errorf("failed to build nested aggregations for '%s': %w", name, err)
			}
			for subName, subAgg := range nestedAggs {
				agg.AddNested(subName, subAgg)
			}
		}

		result[name] = agg
	}

	return result, nil
}
