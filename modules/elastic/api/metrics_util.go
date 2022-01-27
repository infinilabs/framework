package api

import (
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	log "src/github.com/cihub/seelog"
	"strings"
	"time"
)

func newMetricItem(metricKey string, order int, group string) *common.MetricItem {
	metricItem := common.MetricItem{
		Order: order,
		Key:   metricKey,
		Group: group,
	}

	//axis
	metricItem.Axis = []*common.MetricAxis{}

	//lines
	metricItem.Lines = []*common.MetricLine{}

	return &metricItem
}

type GroupMetricItem struct {
	Key          string
	Field        string
	ID           string
	IsDerivative bool
	Units        string
	FormatType   string
	MetricItem   *common.MetricItem
	Field2       string
	Calc         func(value, value2 float64) float64
}

type TreeMapNode struct {
	Name string `json:"name"`
	Value float64 `json:"value,omitempty"`
	Children []*TreeMapNode  `json:"children,omitempty"`
	SubKeys  map[string]int `json:"-"`
}

type MetricData map[string][][]interface{}

func (h *APIHandler) getMetrics(query map[string]interface{}, grpMetricItems []GroupMetricItem, bucketSize int) map[string]*common.MetricItem {
	bucketSizeStr := fmt.Sprintf("%vs", bucketSize)
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
	}
	grpMetricItemsIndex := map[string]int{}
	for i, item := range grpMetricItems {
		grpMetricItemsIndex[item.ID] = i
	}
	grpMetricData := map[string]MetricData{}

	var minDate, maxDate int64
	if response.StatusCode == 200 {
		if nodeAgg, ok := response.Aggregations["group_by_level"]; ok {
			for _, bucket := range nodeAgg.Buckets {
				grpKey := bucket["key"].(string)
				for _, metricItem := range grpMetricItems {
					metricItem.MetricItem.AddLine(metricItem.Key, grpKey, "", "group1", metricItem.Field, "max", bucketSizeStr, metricItem.Units, metricItem.FormatType, "0.[00]", "0.[00]", false, false)
					dataKey := metricItem.ID
					if metricItem.IsDerivative {
						dataKey = dataKey + "_deriv"
					}
					if _, ok := grpMetricData[dataKey]; !ok {
						grpMetricData[dataKey] = map[string][][]interface{}{}
					}
					grpMetricData[dataKey][grpKey] = [][]interface{}{}
				}
				if datesAgg, ok := bucket["dates"].(map[string]interface{}); ok {
					if datesBuckets, ok := datesAgg["buckets"].([]interface{}); ok {
						for _, dateBucket := range datesBuckets {
							if bucketMap, ok := dateBucket.(map[string]interface{}); ok {
								v, ok := bucketMap["key"].(float64)
								if !ok {
									panic("invalid bucket key")
								}
								dateTime := (int64(v))
								minDate = util.MinInt64(minDate, dateTime)
								maxDate = util.MaxInt64(maxDate, dateTime)

								for mk1, mv1 := range grpMetricData {
									v1, ok := bucketMap[mk1]
									if ok {
										v2, ok := v1.(map[string]interface{})
										if ok {
											v3, ok := v2["value"].(float64)
											if ok {
												if strings.HasSuffix(mk1, "_deriv") {
													v3 = v3 / float64(bucketSize)
												}
												if field2, ok := bucketMap[mk1+"_field2"]; ok {
													if idx, ok := grpMetricItemsIndex[mk1]; ok {
														if field2Map, ok := field2.(map[string]interface{}); ok {
															v3 = grpMetricItems[idx].Calc(v3, field2Map["value"].(float64))
														}
													}
												}
												if v3 < 0 {
													continue
												}
												points := []interface{}{dateTime, v3}
												mv1[grpKey] = append(mv1[grpKey], points)
											}
										}
									}
								}
							}
						}
					}

				}
			}
		}
	}

	result := map[string]*common.MetricItem{}

	for _, metricItem := range grpMetricItems {
		for _, line := range metricItem.MetricItem.Lines {
			line.TimeRange = common.TimeRange{Min: minDate, Max: maxDate}
			dataKey := metricItem.ID
			if metricItem.IsDerivative {
				dataKey = dataKey + "_deriv"
			}
			line.Data = grpMetricData[dataKey][line.Metric.Label]
		}
		result[metricItem.Key] = metricItem.MetricItem
	}
	return result
}

//defaultBucketSize 也就是每次聚合的时间间隔
func (h *APIHandler) getMetricRangeAndBucketSize(req *http.Request, defaultBucketSize, defaultMetricCount int) (int, int64, int64, error) {
	if defaultBucketSize <= 0 {
		defaultBucketSize = 10
	}
	if defaultMetricCount <= 0 {
		defaultMetricCount = 15 * 60
	}

	bucketSize := h.GetIntOrDefault(req, "bucket_size", defaultBucketSize)    //默认 10，每个 bucket 的时间范围，单位秒
	metricCount := h.GetIntOrDefault(req, "metric_count", defaultMetricCount) //默认 15分钟的区间，每分钟15个指标，也就是 15*6 个 bucket //90

	now := time.Now()
	//min,max are unix nanoseconds

	minStr := h.Get(req, "min", "")
	maxStr := h.Get(req, "max", "")

	var min, max int64
	var rangeFrom, rangeTo time.Time
	var err error
	var useMinMax bool
	if minStr == "" {
		rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
	} else {
		//try 2021-08-21T14:06:04.818Z
		rangeFrom, err = util.ParseStandardTime(minStr)
		if err != nil {
			//try 1629637500000
			v, err := util.ToInt64(minStr)
			if err != nil {
				log.Error("invalid timestamp:", minStr, err)
				rangeFrom = now.Add(-time.Second * time.Duration(bucketSize*metricCount+1))
			} else {
				rangeFrom = util.FromUnixTimestamp(v / 1000)
			}
		}
		useMinMax = true
	}

	if maxStr == "" {
		rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
	} else {
		rangeTo, err = util.ParseStandardTime(maxStr)
		if err != nil {
			v, err := util.ToInt64(maxStr)
			if err != nil {
				log.Error("invalid timestamp:", maxStr, err)
				rangeTo = now.Add(-time.Second * time.Duration(int(1*(float64(bucketSize)))))
			} else {
				rangeTo = util.FromUnixTimestamp(int64(v) / 1000)
			}
		}
	}

	min = rangeFrom.UnixNano() / 1e6
	max = rangeTo.UnixNano() / 1e6
	hours := rangeTo.Sub(rangeFrom).Hours()

	if useMinMax {

		if hours <= 1 {
			bucketSize = 60
		} else if hours < 3 {
			bucketSize = 90
		} else if hours < 6 {
			bucketSize = 120
		} else if hours < 12 {
			bucketSize = 60 * 3
		} else if hours < 25 { //1day
			bucketSize = 60 * 5 * 2
		} else if hours <= 7*24+1 { //7days
			bucketSize = 60 * 15 * 2
		} else if hours <= 15*24+1 { //15days
			bucketSize = 60 * 30 * 2
		} else if hours < 30*24+1 { //<30 days
			bucketSize = 60 * 60 //hourly
		} else if hours <= 30*24+1 { //<30days
			bucketSize = 12 * 60 * 60 //half daily
		} else if hours >= 30*24+1 { //>30days
			bucketSize = 60 * 60 * 24 //daily bucket
		}
	}

	return bucketSize, min, max, nil
}

//获取单个指标，可以包含多条曲线
func (h *APIHandler) getSingleMetrics(metricItems []*common.MetricItem, query map[string]interface{}, bucketSize int) map[string]*common.MetricItem {
	metricData := map[string][][]interface{}{}

	aggs := map[string]interface{}{}

	for _, metricItem := range metricItems {
		for _, line := range metricItem.Lines {

			metricData[line.Metric.DataKey] = [][]interface{}{}

			aggs[line.Metric.ID] = util.MapStr{
				"max": util.MapStr{
					"field": line.Metric.Field,
				},
			}

			if line.Metric.IsDerivative {
				//add which metric keys to extract
				aggs[line.Metric.ID+"_deriv"] = util.MapStr{
					"derivative": util.MapStr{
						"buckets_path": line.Metric.ID,
					},
				}
			}
		}
	}
	bucketSizeStr := fmt.Sprintf("%vs", bucketSize)

	query["size"] = 0
	query["aggs"] = util.MapStr{
		"dates": util.MapStr{
			"date_histogram": util.MapStr{
				"field":          "timestamp",
				"fixed_interval": bucketSizeStr,
			},
			"aggs": aggs,
		},
	}
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
		panic(err)
	}

	var minDate, maxDate int64
	if response.StatusCode == 200 {
		for _, v := range response.Aggregations {
			for _, bucket := range v.Buckets {
				v, ok := bucket["key"].(float64)
				if !ok {
					panic("invalid bucket key")
				}
				dateTime := (int64(v))
				minDate = util.MinInt64(minDate, dateTime)
				maxDate = util.MaxInt64(maxDate, dateTime)
				for mk1, mv1 := range metricData {
					v1, ok := bucket[mk1]
					if ok {
						v2, ok := v1.(map[string]interface{})
						if ok {
							v3, ok := v2["value"].(float64)
							if ok {
								if strings.HasSuffix(mk1, "_deriv") {
									v3 = v3 / float64(bucketSize)
								}
								//only keep positive value
								if v3 < 0 {
									continue
								}
								//v4:=int64(v3)/int64(bucketSize)
								points := []interface{}{dateTime, v3}
								metricData[mk1] = append(mv1, points)
							}
						}
					}
				}
			}
		}
	}

	result := map[string]*common.MetricItem{}

	for _, metricItem := range metricItems {
		for _, line := range metricItem.Lines {
			line.TimeRange = common.TimeRange{Min: minDate, Max: maxDate}
			line.Data = metricData[line.Metric.DataKey]
		}
		result[metricItem.Key] = metricItem
	}

	return result
}

//
func (h *APIHandler) getGroupMetrics(query map[string]interface{}) {
	//response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))

}

func (h *APIHandler) convertMetricItemsToAgg(metricItems []*common.MetricItem)map[string]interface{} {
	aggs := map[string]interface{}{}
	for _, metricItem := range metricItems {
		for _, line := range metricItem.Lines {
			aggs[line.Metric.ID] = util.MapStr{
				"max": util.MapStr{
					"field": line.Metric.Field,
				},
			}
			if line.Metric.IsDerivative {
				//add which metric keys to extract
				aggs[line.Metric.ID+"_deriv"] = util.MapStr{
					"derivative": util.MapStr{
						"buckets_path": line.Metric.ID,
					},
				}
			}
		}
	}
	return aggs
}
