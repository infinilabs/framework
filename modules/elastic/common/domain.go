/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package common

import (
	"infini.sh/framework/core/util"
)

const PositionLeft = "left"
const PositionRight = "right"
const PositionTop = "top"
const PositionBottom = "bottom"

type MetricAxis struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Title string `json:"title"`

	FormatType    string `json:"formatType"`
	Position      string `json:"position"`
	TickFormat    string `json:"tickFormat"`
	Ticks         int    `json:"ticks"`
	LabelFormat   string `json:"labelFormat"`
	ShowGridLines bool   `json:"showGridLines"`
}

type TimeRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type MetricLine struct {
	TimeRange  TimeRange       `json:"timeRange"`
	Data       [][]interface{} `json:"data"`
	BucketSize string          `json:"bucket_size"`
	Metric     MetricSummary   `json:"metric"`
}

type MetricSummary struct {

	//App string `json:"app"`

	Group       string `json:"group"`
	Title       string `json:"title"`
	Label       string `json:"label"`
	Description string `json:"description"`

	ID        string `json:"-"`
	DataKey   string `json:"-"`
	MetricAgg string `json:"metricAgg"`
	Field     string `json:"field"`

	FormatType string `json:"formatType"`
	Format     string `json:"format"`
	TickFormat string `json:"tickFormat"`
	Units      string `json:"units"`

	HasCalculation bool `json:"hasCalculation"`
	IsDerivative   bool `json:"isDerivative"`
}

type MetricItem struct {
	Key   string        `json:"key"`
	Axis  []*MetricAxis `json:"axis"`
	Lines []*MetricLine `json:"lines"`
	Group string `json:"group"`
	Order int `json:"order"`
}

func (metricItem *MetricItem) AddLine(title, label, desc, group, field, aggsType, bucketSize, units, formatTye, format, tickFormat string, hasCalculation, isDerivative bool) *MetricItem {
	line := MetricLine{}
	line.BucketSize = bucketSize
	line.Metric = MetricSummary{
		ID: util.GetUUID(),
		//App: "elasticsearch",
		Title:          title,
		Label:          label,
		Description:    desc,
		Group:          group,
		Field:          field,
		MetricAgg:      aggsType,
		Units:          units,
		FormatType:     formatTye,
		Format:         format,
		TickFormat:     tickFormat,
		HasCalculation: hasCalculation,
		IsDerivative:   isDerivative,
	}

	if line.Metric.IsDerivative {
		line.Metric.DataKey = line.Metric.ID + "_deriv"
	} else {
		line.Metric.DataKey = line.Metric.ID
	}

	metricItem.Lines = append(metricItem.Lines, &line)
	return metricItem
}

func (metricItem *MetricItem) AddAxi(title, group, position, formatType, labelFormat, tickFormat string, ticks int, showGridLines bool) *MetricItem {
	axis := MetricAxis{}
	axis.ID = util.GetUUID()
	axis.Title = title
	axis.Group = group
	axis.Position = position
	axis.FormatType = formatType
	axis.LabelFormat = labelFormat
	axis.TickFormat = tickFormat
	axis.Ticks = ticks
	axis.ShowGridLines = showGridLines

	//axis
	metricItem.Axis = append(metricItem.Axis, &axis)

	return metricItem
}

