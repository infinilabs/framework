package elastic

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
)

var NEWLINEBYTES = []byte("\n")

type BulkResult struct {
	Error     bool        `json:"error,omitempty"`
	ErrorMsgs []string    `json:"error_msgs,omitempty"`
	Codes     []int       `json:"codes,omitempty"`
	Indices   []string    `json:"indices,omitempty"`
	Actions   []string    `json:"actions,omitempty"`
	Summary   BulkSummary `json:"summary,omitempty"`
	Stats     BulkStats   `json:"stats,omitempty"`
	Detail    BulkDetail  `json:"detail,omitempty"`
}

type BulkSummary struct {
	Failure BulkSummaryItem `json:"failure,omitempty"`
	Invalid BulkSummaryItem `json:"invalid,omitempty"`
	Success BulkSummaryItem `json:"success,omitempty"`
}

type BulkSummaryItem struct {
	Count int `json:"count,omitempty"`
	Size  int `json:"size,omitempty"`
}

type BulkStats struct {
	Code    map[int]int    `json:"code,omitempty"`
	Indices map[string]int `json:"indices,omitempty"`
	Actions map[string]int `json:"actions,omitempty"`
}

type BulkDetail struct {
	Failure BulkDetailItem `json:"failure,omitempty"`
	Invalid BulkDetailItem `json:"invalid,omitempty"`
}

type BulkDetailItem struct {
	Documents []string `json:"documents,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
}

func WalkBulkRequests(data []byte, eachLineFunc func(eachLine []byte) (skipNextLine bool), metaFunc func(metaBytes []byte, actionStr, index, typeName, id, routing string, offset int) (err error), payloadFunc func(payloadBytes []byte, actionStr, index, typeName, id, routing string)) (int, error) {

	nextIsMeta := true
	skipNextLineProcessing := false
	var docCount = 0

	lines := bytes.Split(data, NEWLINEBYTES)
	//reset
	nextIsMeta = true
	skipNextLineProcessing = false
	docCount = 0

	var actionStr string
	var index string
	var typeName string
	var id string
	var routing string

	for i, line := range lines {

		bytesCount := len(line)
		if line == nil || bytesCount <= 0 {
			if global.Env().IsDebug {
				log.Tracef("invalid line, continue, [%v/%v] [%v]\n%v", i, len(lines), string(line), util.PrintStringByteLines(lines))
			}
			continue
		}

		if eachLineFunc != nil {
			skipNextLineProcessing = eachLineFunc(line)
		}

		if skipNextLineProcessing {
			skipNextLineProcessing = false
			nextIsMeta = true
			log.Debug("skip body processing")
			continue
		}

		if nextIsMeta {
			nextIsMeta = false
			var err error
			actionStr, index, typeName, id, routing, err = ParseActionMeta(line)
			if global.Env().IsDebug {
				log.Trace(docCount, ",", actionStr, index, typeName, id, routing, err, ",", string(line))
			}
			if err != nil {
				if global.Env().IsDebug {
					if util.ContainStr(err.Error(), "invalid_meta_buffer") {
						if i > 0 {
							previous := lines[i-1]
							log.Info("line:", i, ",previous line:", string(previous), ",raw:", string(lines[i]), ",invalid:", string(line), ",full message:", string(data))
						}
					}
				}
				panic(err)
			}

			err = metaFunc(line, actionStr, index, typeName, id, routing, docCount)
			if err != nil {
				panic(err)
			}

			docCount++

			if actionStr == ActionDelete {
				nextIsMeta = true
			}
		} else {
			nextIsMeta = true
			payloadFunc(line, actionStr, index, typeName, id, routing)
		}
	}

	if global.Env().IsDebug {
		log.Tracef("total [%v] operations in bulk requests", docCount)
	}

	return docCount, nil
}

func ParseUrlLevelBulkMeta(pathStr string) (urlLevelIndex, urlLevelType string) {

	if !util.SuffixStr(pathStr, "_bulk") {
		return urlLevelIndex, urlLevelType
	}

	if !util.PrefixStr(pathStr, "/") {
		return urlLevelIndex, urlLevelType
	}

	if strings.Index(pathStr, "//") >= 0 {
		pathStr = strings.ReplaceAll(pathStr, "//", "/")
	}

	if strings.LastIndex(pathStr, "/") == 0 {
		return urlLevelIndex, urlLevelType
	}

	pathArray := strings.Split(pathStr, "/")

	switch len(pathArray) {
	case 4:
		urlLevelIndex = pathArray[1]
		urlLevelType = pathArray[2]
		break
	case 3:
		urlLevelIndex = pathArray[1]
		break
	}

	return urlLevelIndex, urlLevelType
}

type BulkProcessorConfig struct {
	bulkSizeInByte int

	BulkSizeInKb     int `config:"batch_size_in_kb,omitempty"`
	BulkSizeInMb     int `config:"batch_size_in_mb,omitempty"`
	BulkMaxDocsCount int `config:"batch_size_in_docs,omitempty"`

	Compress               bool `config:"compress"`
	RetryDelayInSeconds    int  `config:"retry_delay_in_seconds"`
	RejectDelayInSeconds   int  `config:"reject_retry_delay_in_seconds"`
	MaxRejectRetryTimes    int  `config:"max_reject_retry_times"`
	RequestTimeoutInSecond int  `config:"request_timeout_in_second"`

	InvalidRequestsQueue    string `config:"invalid_queue"`
	DeadletterRequestsQueue string `config:"dead_letter_queue"`

	RetryRules RetryRules `config:"retry_rules"`

	BulkResponseParseConfig BulkResponseParseConfig `config:"response_handle"`

	RemoveDuplicatedNewlines bool `config:"remove_duplicated_newlines"`
}

type BulkResponseParseConfig struct {
	SaveSuccessBulkResultToMessageQueue bool `config:"save_success_results"`
	SaveErrorBulkResultToMessageQueue   bool `config:"save_error_results"`
	SaveBusyBulkResultToMessageQueue    bool `config:"save_busy_results"`

	OutputBulkStats    bool `config:"output_bulk_stats"`
	IncludeIndexStats  bool `config:"include_index_stats"`
	IncludeActionStats bool `config:"include_action_stats"`

	IncludeErrorDetails        bool `config:"include_error_details"`
	MaxItemOfErrorDetailsCount int  `config:"max_error_details_count"`

	BulkResultMessageQueue                 string `config:"bulk_result_message_queue"`
	BulkResultMessageMaxRequestBodyLength  int    `config:"max_request_body_size"`
	BulkResultMessageMaxResponseBodyLength int    `config:"max_response_body_size"`
}

type RetryRule struct {
	//response
	Status  []int    `config:"status"`
	Keyword []string `config:"keyword"`
}

type RetryRules struct {
	//Retry3xx        bool     `config:"retry_3xx"`
	Retry4xx bool `config:"retry_4xx"`
	Retry429 bool `config:"retry_429"`

	Default bool `config:"default"`

	Permitted RetryRule `config:"permitted"`
	Denied    RetryRule `config:"denied"`
}

func (this *RetryRules) Retryable(code int, msg string) bool {

	//allow
	if len(this.Permitted.Status) > 0 {
		for _, v := range this.Permitted.Status {
			if v == code {
				return true
			}
		}
	}

	if len(this.Permitted.Keyword) > 0 && len(msg) > 0 {
		for _, v := range this.Permitted.Keyword {
			if util.ContainStr(msg, v) {
				log.Debug("message: ", msg, " contains keyword: [", v, "], allow retry")
				return true
			}
		}
	}

	//deny
	if len(this.Denied.Status) > 0 {
		for _, v := range this.Denied.Status {
			if v == code {
				return false
			}
		}
	}

	if len(this.Denied.Keyword) > 0 && len(msg) > 0 {
		for _, v := range this.Denied.Keyword {
			if util.ContainStr(msg, v) {
				log.Debug("message: ", msg, " contains keyword: [", v, "], skip retry")
				return false
			}
		}
	}

	//handle 4xx
	if code >= 400 && code < 500 { //find invalid request 409
		if code == 429 {
			return this.Retry429
		}
		return this.Retry4xx
	}

	return this.Default
}

func (this *BulkProcessorConfig) GetBulkSizeInBytes() int {

	this.bulkSizeInByte = 1048576 * this.BulkSizeInMb
	if this.BulkSizeInKb > 0 {
		this.bulkSizeInByte = 1024 * this.BulkSizeInKb
	}
	if this.bulkSizeInByte <= 0 {
		this.bulkSizeInByte = 10 * 1024 * 1024
	}
	return this.bulkSizeInByte
}

var DefaultBulkProcessorConfig = BulkProcessorConfig{
	BulkMaxDocsCount:     1000,
	BulkSizeInMb:         10,
	Compress:             false,
	RetryDelayInSeconds:  1,
	RejectDelayInSeconds: 1,
	MaxRejectRetryTimes:  0,

	DeadletterRequestsQueue: "bulk_dead_requests",
	BulkResponseParseConfig: BulkResponseParseConfig{
		BulkResultMessageMaxRequestBodyLength:  10 * 1024,
		BulkResultMessageMaxResponseBodyLength: 10 * 1024,

		BulkResultMessageQueue: "bulk_result_messages",

		SaveSuccessBulkResultToMessageQueue: false,
		SaveErrorBulkResultToMessageQueue:   true,
		SaveBusyBulkResultToMessageQueue:    true,
		OutputBulkStats:                     false,

		IncludeErrorDetails:        true,
		IncludeIndexStats:          true,
		IncludeActionStats:         true,
		MaxItemOfErrorDetailsCount: 50,
	},
	RetryRules:             RetryRules{Retry429: true, Default: true, Retry4xx: false},
	RequestTimeoutInSecond: 60,
}

type BulkProcessor struct {
	Config BulkProcessorConfig
}

// bulkResult is valid only if max_reject_retry_times == 0
func (joint *BulkProcessor) Bulk(ctx context.Context, tag string, metadata *ElasticsearchMetadata, host string, buffer *BulkBuffer) (continueNext bool, statsRet map[int]int, bulkResult *BulkResult, err error) {

	statsRet = make(map[int]int)

	if buffer == nil || buffer.GetMessageSize() == 0 {
		return true, statsRet, nil, errors.New("invalid or empty message")
	}

	host = metadata.GetActivePreferredHost(host)

	if host == "" {
		panic("invalid host")
	}

	httpClient := metadata.GetHttpClient(host)

	var url string
	if metadata.IsTLS() {
		url = fmt.Sprintf("https://%s/_bulk", host)
	} else {
		url = fmt.Sprintf("http://%s/_bulk", host)
	}

	req := fasthttp.AcquireRequestWithTag("bulk_processing_request")
	resp := fasthttp.AcquireResponseWithTag("bulk_processing_response")
	defer fasthttp.ReleaseRequest(req)   // <- do not forget to release
	defer fasthttp.ReleaseResponse(resp) // <- do not forget to release

	req.SetRequestURI(url)

	req.Header.SetMethod(http.MethodPost)
	req.Header.SetUserAgent("_bulk")
	req.Header.SetContentType("application/x-ndjson")

	clonedURI := req.CloneURI()
	defer fasthttp.ReleaseURI(clonedURI)

	if metadata.Config.BasicAuth != nil {
		clonedURI.SetUsername(metadata.Config.BasicAuth.Username)
		clonedURI.SetPassword(metadata.Config.BasicAuth.Password)
	}

	//acceptGzipped := req.AcceptGzippedResponse()
	//compressed := false

	// handle last \n
	buffer.SafetyEndWithNewline()
	data := buffer.GetMessageBytes()

	if joint.Config.RemoveDuplicatedNewlines {
		//handle double \n
		data = bytes.ReplaceAll(data, []byte("\n\n"), []byte("\n"))
	}

	if !req.IsGzipped() && joint.Config.Compress {

		_, err := fasthttp.WriteGzipLevel(req.BodyWriter(), data, fasthttp.CompressBestSpeed)
		if err != nil {
			panic(err)
		}

		//TODO handle response, if client not support gzip, return raw body
		req.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
		req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
		//compressed = true

	} else {
		req.SetBody(data)
	}

	// modify schema，align with elasticsearch's schema
	orignalSchema := string(clonedURI.Scheme())
	orignalHost := string(clonedURI.Host())

	if host != "" && req.Host() == nil || string(req.Host()) != orignalHost {
		req.Header.SetHost(host)
	}

	if metadata.GetSchema() != orignalSchema {
		clonedURI.SetScheme(metadata.GetSchema())
	}

	retryTimes := 0
	nonRetryableItems := AcquireBulkBuffer()
	retryableItems := AcquireBulkBuffer()
	successItems := AcquireBulkBuffer()

	defer ReturnBulkBuffer(nonRetryableItems)
	defer ReturnBulkBuffer(retryableItems)
	defer ReturnBulkBuffer(successItems)

DO:

	if req.GetBodyLength() <= 0 {
		panic(errors.Error("request body is zero,", len(data), ",is compress:", joint.Config.Compress))
	}

	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(req.Header.Host()), 1, req.GetRequestLength(), 0)

	stats.Increment("elasticsearch.bulk", "submit")

	req.SetURI(clonedURI)
	//execute
	err = httpClient.DoTimeout(req, resp, time.Duration(joint.Config.RequestTimeoutInSecond)*time.Second)
	//restore schema
	clonedURI.SetScheme(orignalSchema)
	req.SetURI(clonedURI)
	req.SetHost(orignalHost)

	if err != nil {
		if rate.GetRateLimiter(metadata.Config.ID, host+"5xx_on_error", 1, 1, 5*time.Second).Allow() {
			log.Error("status:", resp.StatusCode(), ",", host, ",", err, " ", util.SubString(util.UnsafeBytesToString(resp.GetRawBody()), 0, 256))
			time.Sleep(2 * time.Second)
		}
		return false, statsRet, nil, err
	}

	////restore body and header
	//if !acceptGzipped && compressed {
	//
	//	body := resp.GetRawBody()
	//
	//	resp.SwapBody(body)
	//
	//	resp.Header.Del(fasthttp.HeaderContentEncoding)
	//	resp.Header.Del(fasthttp.HeaderContentEncoding2)
	//
	//}

	if resp == nil {
		if global.Env().IsDebug {
			log.Error(err)
		}
		return false, statsRet, nil, err
	}

	// Do we need to decompress the response?
	var resbody = resp.GetRawBody()

	if global.Env().IsDebug {
		log.Trace(resp.StatusCode(), util.UnsafeBytesToString(util.EscapeNewLine(data)), util.UnsafeBytesToString(util.EscapeNewLine(resbody)))
	}

	//for response
	labels := util.MapStr{
		"cluster_id": metadata.Config.ID,
		"host":       orignalHost,
		"queue":      buffer.Queue,
	}
	if retryTimes > 0 {
		labels["retry_times"] = retryTimes
	}

	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusCreated {

		//如果是部分失败，应该将可以重试的做完，然后记录失败的消息再返回不继续
		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {

			containError, statsCodeStats, bulkResult := HandleBulkResponse(req, resp, labels, data, resbody, successItems, nonRetryableItems, retryableItems, joint.Config.BulkResponseParseConfig, joint.Config.RetryRules)

			for k, v := range statsCodeStats {
				if global.Env().IsDebug {
					stats.IncrementBy("bulk::"+tag, util.ToString(k), int64(v))
				}
				statsRet[k] = statsRet[k] + v
			}

			if retryTimes > 0 || global.Env().IsDebug {
				log.Infof("#%v, code:%v, contain_err:%v, status:%v, success:%v, failure:%v, invalid:%v, result:%+v",
					retryTimes, resp.StatusCode(), containError, statsCodeStats, successItems.GetMessageCount(), retryableItems.GetMessageCount(), nonRetryableItems.GetMessageCount(), bulkResult)
			}

			if containError {
				count := retryableItems.GetMessageCount()
				if count > 0 {
					log.Debugf("%v, retry item: %v", tag, count)
					retryableItems.SafetyEndWithNewline()
					bodyBytes := retryableItems.GetMessageBytes()
					req.SetRawBody(bodyBytes)
					delayTime := joint.Config.RejectDelayInSeconds

					if delayTime <= 0 {
						delayTime = 5
					}

					time.Sleep(time.Duration(delayTime) * time.Second)

					if joint.Config.MaxRejectRetryTimes < 0 {
						joint.Config.MaxRejectRetryTimes = 3
					}

					if retryTimes >= joint.Config.MaxRejectRetryTimes {

						//continue retry before is back
						if !metadata.IsAvailable() {
							return false, statsRet, bulkResult, errors.Errorf("elasticsearch [%v] is not available", metadata.Config.Name)
						}

						data := req.OverrideBodyEncode(bodyBytes, true)
						queue.Push(queue.GetOrInitConfig(metadata.Config.ID+"_dead_letter_queue"), data)
						return true, statsRet, bulkResult, errors.Errorf("bulk partial failure, retried %v times, quit retry", retryTimes)
					}
					log.Infof("%v, bulk partial failure, #%v retry, %v items left, size: %v, stats:%v", tag, retryTimes, retryableItems.GetMessageCount(), retryableItems.GetMessageSize(), statsCodeStats)
					retryTimes++
					stats.Increment("elasticsearch."+tag+"."+metadata.Config.Name+".bulk", "retry")

					goto DO
				}

				var continueNext = false
				if retryableItems.GetMessageCount() == 0 {
					// no retryable
					continueNext = true
				}
				if nonRetryableItems.GetMessageCount() > 0 {
					continueNext = false
					////handle 400 error
					if joint.Config.InvalidRequestsQueue != "" {
						queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
						continueNext = true
					} else {
						return continueNext, statsRet, bulkResult, errors.New("bulk contains invalid requests, but invalid_queue is not configured")
					}
				}

				return continueNext, statsRet, bulkResult, nil
			}
			return true, statsRet, bulkResult, nil
		}
		return true, statsRet, bulkResult, nil
	} else {
		statsRet[resp.StatusCode()] = statsRet[resp.StatusCode()] + buffer.GetMessageCount()

		var bulkResult *BulkResult

		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {
			_, _, bulkResult = HandleBulkResponse(req, resp, labels, data, resbody, successItems, nonRetryableItems, retryableItems, joint.Config.BulkResponseParseConfig, joint.Config.RetryRules)
		}

		if resp.StatusCode() == 429 {
			return false, statsRet, bulkResult, errors.Errorf("code 429, [%v] is too busy", metadata.Config.Name)
		} else if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			////handle 400 error
			if joint.Config.InvalidRequestsQueue != "" {
				queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
				return true, statsRet, bulkResult, nil
			}
			return false, statsRet, bulkResult, errors.Errorf("invalid requests, code: %v", resp.StatusCode())
		} else {

			if global.Env().IsDebug {
				log.Debugf("status:", resp.StatusCode(), ",request:", util.UnsafeBytesToString(req.GetRawBody()), ",response:", util.UnsafeBytesToString(resbody))
			}

			if !joint.Config.RetryRules.Retryable(resp.StatusCode(), util.UnsafeBytesToString(resp.GetRawBody())) {
				truncatedResponse := util.SubString(util.UnsafeBytesToString(resbody), 0, joint.Config.BulkResponseParseConfig.BulkResultMessageMaxRequestBodyLength)
				log.Warnf("code: %v, but hit condition to skip retry, response: %v", resp.StatusCode(), truncatedResponse)
				return true, statsRet, bulkResult, errors.Errorf("code: %v, response: %v", resp.StatusCode(), truncatedResponse)
			}

			return false, statsRet, bulkResult, errors.Errorf("request failed, code: %v", resp.StatusCode())
		}
	}
}

func HandleBulkResponse(req *fasthttp.Request, resp *fasthttp.Response, tag util.MapStr, requestBytes, resbody []byte, successItems *BulkBuffer, nonRetryableItems, retryableItems *BulkBuffer, options BulkResponseParseConfig, retryRules RetryRules) (bool, map[int]int, *BulkResult) {
	nonRetryableItems.ResetData()
	retryableItems.ResetData()
	successItems.ResetData()

	containError := util.LimitedBytesSearch(resbody, []byte("\"errors\":true"), 64)
	var statsCodeStats = map[int]int{}
	var reqFailed bool
	invalidDocStatus := map[int]int{}
	invalidDocError := map[int]string{}
	invalidDocID := map[int]string{}

	var errorMsgs []string
	reqError, _, _, err := jsonparser.Get(resbody, "error")
	if err == nil {
		reqFailed = true
		errorMsgs = append(errorMsgs, string(reqError))
	}

	var indexStatsData map[string]int = map[string]int{}
	var actionStatsData map[string]int = map[string]int{}

	items, _, _, err := jsonparser.Get(resbody, "items")
	if err == nil {
		var docOffset = 0
		jsonparser.ArrayEach(items, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			item, _, _, err := jsonparser.Get(value, "index")
			if err != nil {
				item, _, _, err = jsonparser.Get(value, "delete")
				if err != nil {
					item, _, _, err = jsonparser.Get(value, "create")
					if err != nil {
						item, _, _, err = jsonparser.Get(value, "update")
					}
				}
			}
			if err == nil {

				//id can be nil
				docId, err := jsonparser.GetUnsafeString(item, "_id")
				if err != nil {
					log.Error(docId, err)
				}
				if err == nil {
					docId = strings.Clone(docId) //TODO
					//docId=docId //TODO
				}

				code1, err := jsonparser.GetInt(item, "status")
				if err != nil {
					panic(err)
				}
				code := int(code1)
				x, ok := statsCodeStats[code]
				if !ok {
					x = 0
				}
				x++
				statsCodeStats[code] = x
				erObj, _, _, err := jsonparser.Get(item, "error")
				if err == nil && erObj != nil {
					invalidDocID[docOffset] = docId
					invalidDocStatus[docOffset] = code
					invalidDocError[docOffset] = string(erObj)
				}
			}
			docOffset++
		})

		if global.Env().IsDebug {
			log.Debug(tag, " bulk status:", statsCodeStats, ",invalid status:", invalidDocStatus)
		}
	}

	var match = false
	var retryable = false
	WalkBulkRequests(requestBytes, func(eachLine []byte) (skipNextLine bool) {
		return false
	}, func(metaBytes []byte, actionStr, index, typeName, id, routing string, offset int) (err error) {
		if reqFailed {
			nonRetryableItems.WriteMessageID(fmt.Sprintf("N/A(%v)", offset))
			return nil
		}

		if options.IncludeIndexStats {
			//init
			if indexStatsData == nil {
				if indexStatsData == nil {
					indexStatsData = map[string]int{}
				}
			}

			//stats
			indexName := RemoveDotFromIndexName(index, "#")
			v, ok := indexStatsData[indexName]
			if !ok {
				indexStatsData[indexName] = 1
			} else {
				indexStatsData[indexName] = v + 1
			}
		}

		if options.IncludeActionStats {
			//init
			if actionStatsData == nil {
				if actionStatsData == nil {
					actionStatsData = map[string]int{}
				}
			}

			//stats
			v, ok := actionStatsData[actionStr]
			if !ok {
				actionStatsData[actionStr] = 1
			} else {
				actionStatsData[actionStr] = v + 1
			}
		}

		var code int
		code, match = invalidDocStatus[offset]
		if id == "" {
			id = fmt.Sprintf("N/A(%v)", offset)
		}
		if match {

			docID := invalidDocID[offset]
			if docID == "" {
				docID = fmt.Sprintf("N/A(%v)", offset)
			}

			//check
			errMsg := invalidDocError[offset]
			retryable = retryRules.Retryable(code, errMsg)

			if global.Env().IsDebug {
				log.Debugf("index:%v, offset:%v, docID:%v, action:%v, code:%v, retryable:%v, reason:%v", index, offset, docID, actionStr, code, retryable, errMsg)
			}

			if retryable {
				retryableItems.WriteNewByteBufferLine("retryable", metaBytes)
				retryableItems.WriteMessageID(docID)
				retryableItems.WriteErrorReason(errMsg)
			} else {
				nonRetryableItems.WriteNewByteBufferLine("none-retryable", metaBytes)
				nonRetryableItems.WriteMessageID(docID)
				nonRetryableItems.WriteErrorReason(errMsg)
			}
		} else {
			successItems.WriteNewByteBufferLine("success", metaBytes)
			successItems.WriteMessageID(id)
		}
		return nil
	}, func(payloadBytes []byte, actionStr, index, typeName, id, routing string) {
		if reqFailed {
			return
		}
		if match {
			if payloadBytes != nil && len(payloadBytes) > 0 {
				if retryable {
					retryableItems.WriteNewByteBufferLine("retryable", payloadBytes)
				} else {
					nonRetryableItems.WriteNewByteBufferLine("none-retryable", payloadBytes)
				}
			}
		} else {
			if payloadBytes != nil && len(payloadBytes) > 0 {
				successItems.WriteNewByteBufferLine("success", payloadBytes)
			}
		}
	})

	//save log and stats
	var bulkResult *BulkResult

	//skip 429 results to response queue
	if resp.StatusCode() == 429 && !options.SaveBusyBulkResultToMessageQueue {
		return containError, statsCodeStats, bulkResult
	}

	if options.OutputBulkStats || options.SaveErrorBulkResultToMessageQueue || options.SaveSuccessBulkResultToMessageQueue {

		bulkResult = &BulkResult{
			Error:     containError,
			ErrorMsgs: errorMsgs,
			Codes:     util.GetIntMapKeys(statsCodeStats),
			Indices:   util.GetStringIntMapKeys(indexStatsData),
			Actions:   util.GetStringIntMapKeys(actionStatsData),
			Summary: BulkSummary{
				Failure: BulkSummaryItem{
					Count: retryableItems.GetMessageCount(),
					Size:  retryableItems.GetMessageSize(),
				},
				Invalid: BulkSummaryItem{
					Count: nonRetryableItems.GetMessageCount(),
					Size:  nonRetryableItems.GetMessageSize(),
				},
				Success: BulkSummaryItem{
					Count: successItems.GetMessageCount(),
					Size:  successItems.GetMessageSize(),
				},
			},
			Stats: BulkStats{
				Code:    statsCodeStats,
				Indices: indexStatsData,
				Actions: actionStatsData,
			},
		}

		if options.IncludeErrorDetails {
			if nonRetryableItems.GetMessageCount() > 0 {
				ids := nonRetryableItems.MessageIDs
				reasons := nonRetryableItems.Reason
				if len(ids) > options.MaxItemOfErrorDetailsCount {
					ids = ids[0:options.MaxItemOfErrorDetailsCount]
				}
				if len(reasons) > options.MaxItemOfErrorDetailsCount {
					reasons = reasons[0:options.MaxItemOfErrorDetailsCount]
				}
				bulkResult.Detail.Invalid = BulkDetailItem{
					Documents: ids,
					Reasons:   reasons,
				}
			}

			if retryableItems.GetMessageCount() > 0 {
				ids := retryableItems.MessageIDs
				reasons := retryableItems.Reason
				if len(ids) > options.MaxItemOfErrorDetailsCount {
					ids = ids[0:options.MaxItemOfErrorDetailsCount]
				}
				if len(reasons) > options.MaxItemOfErrorDetailsCount {
					reasons = reasons[0:options.MaxItemOfErrorDetailsCount]
				}
				bulkResult.Detail.Failure = BulkDetailItem{
					Documents: ids,
					Reasons:   reasons,
				}
			}

		}

		if containError && options.SaveErrorBulkResultToMessageQueue || !containError && options.SaveSuccessBulkResultToMessageQueue {
			if options.BulkResultMessageQueue != "" {
				//save message bytes, with metadata, set codec to wrapped bulk messages
				queue.Push(queue.GetOrInitConfig(options.BulkResultMessageQueue), util.MustToJSONBytes(util.MapStr{
					"timestamp":    time.Now(),
					"bulk_results": bulkResult,
					"labels":       tag,
					"node":         global.Env().SystemConfig.NodeConfig,
					"request": util.MapStr{
						"uri":         req.PhantomURI().String(),
						"body_length": len(requestBytes),
						"body":        util.SubString(util.UnsafeBytesToString(req.GetRawBody()), 0, options.BulkResultMessageMaxRequestBodyLength),
					},
					"response": util.MapStr{
						"status_code": resp.StatusCode(),
						"body_length": len(resbody),
						"body":        util.SubString(util.UnsafeBytesToString(resbody), 0, options.BulkResultMessageMaxResponseBodyLength),
					},
				}))
			}
		}
	}

	return containError, statsCodeStats, bulkResult
}
