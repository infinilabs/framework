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
	Data       interface{} `json:"data"`
	BucketSize string          `json:"bucket_size"`
	Metric     MetricSummary   `json:"metric"`
	Color string `json:"color"`
	Type GraphType `json:"type"`
}

type MetricSummary struct {

	//App string `json:"app"`

	Group       string `json:"group"`
	Title       string `json:"title"`
	Label       string `json:"label"`
	Description string `json:"description"`

	ID        string `json:"-"`
	MetricAgg string `json:"metricAgg"`
	Field     string `json:"field"`

	FormatType string `json:"formatType"`
	Format     string `json:"format"`
	TickFormat string `json:"tickFormat"`
	Units      string `json:"units"`

	HasCalculation bool `json:"hasCalculation"`
	IsDerivative   bool `json:"isDerivative"`
	Field2       string `json:"-"`
	Calc         func(value, value2 float64) float64 `json:"-"`
}

func (receiver *MetricSummary) GetDataKey()string  {
	if receiver.IsDerivative {
		return receiver.ID + "_deriv"
	} else {
		return receiver.ID
	}
}


type MetricItem struct {
	Key   string        `json:"key"`
	Axis  []*MetricAxis `json:"axis"`
	Lines []*MetricLine `json:"lines"`
	Group string `json:"group"`
	Order int `json:"order"`
}

const TermsBucket string="terms"
const DateHistogramBucket string="date_histogram"
const DateRangeBucket string="date_range"

type BucketItem struct {
	Key      string        `json:"key"`
	Group    string        `json:"group"`
	Type    string        ` json:"type"`        //terms/date_histogram
	Parameters    util.MapStr       ` json:"parameters"` //terms/date_histogram
	Order    int           `json:"order"`
	Buckets []*BucketItem  `json:"buckets"`
	Metrics []*MetricItem  `json:"metrics"`
}

func NewBucketItem(bucketType string,parameter util.MapStr)*BucketItem  {
	item:=BucketItem{}
	item.Key=util.GetUUID()
	item.Type=bucketType
	item.Parameters=parameter
	return &item
}

func (bucketItem *BucketItem) AddNestBucket(item  *BucketItem){
	bucketItem.Buckets=append(bucketItem.Buckets,item)
}

func (bucketItem *BucketItem) AddMetricItems(items...  *MetricItem)(*BucketItem){
	for _,i:=range items{
		bucketItem.Metrics=append(bucketItem.Metrics,i)
	}
	return bucketItem
}

func (metricItem *MetricItem) AddLine(title, label, desc, group, field, aggsType, bucketSize, units, formatTye, format, tickFormat string, hasCalculation, isDerivative bool) *MetricLine {
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

	metricItem.Lines = append(metricItem.Lines, &line)
	return &line
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

type GraphType string
const (
	GraphTypeLine GraphType = "Line" //default
	GraphTypeBar GraphType = "Bar"
)
