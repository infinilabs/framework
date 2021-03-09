/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package common

import "time"

const PositionLeft = "left"
const PositionRight = "right"
const PositionTop = "top"
const PositionBottom = "bottom"

type MetricAxis struct{
	ID string  `json:"id"`
	Group string  `json:"group"`
	Title string  `json:"title"`

	FormatType string  `json:"formatType"`
	Position string  `json:"position"`
	TickFormat string  `json:"tickFormat"`
	Ticks int  `json:"ticks"`
	LabelFormat string  `json:"labelFormat"`
	ShowGridLines bool  `json:"showGridLines"`
}

type TimeRange struct{
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type MetricLine struct {
	TimeRange TimeRange`json:"timeRange"`
	Data [][]interface{} `json:"data"`
	BucketSize string `json:"bucket_size"`
	Metric MetricSummary `json:"metric"`
}

type MetricSummary struct {
	App string `json:"app"`
	Group string `json:"group"`
	Title string `json:"title"`
	Label string `json:"label"`
	Description string `json:"description"`

	MetricAgg string `json:"metricAgg"`
	Field string `json:"field"`

	FormatType string `json:"formatType"`
	Format string `json:"format"`
	TickFormat string `json:"tickFormat"`
	Units string `json:"units"`

	HasCalculation bool `json:"hasCalculation"`
	IsDerivative bool `json:"isDerivative"`
}

type MetricItem struct {
	Axis []MetricAxis  `json:"axis"`
	Lines []MetricLine `json:"lines"`
}

type MonitoringItem struct {
	Agent         interface{} `json:"agent,omitempty"`
	Timestamp     time.Time   `json:"timestamp,omitempty" elastic_mapping:"timestamp: { type: date }"`
	Elasticsearch string      `json:"elasticsearch,omitempty"`
	ClusterStats  interface{} `json:"cluster_stats,omitempty"`
}
