package elastic

import (
	"bytes"
	"fmt"
	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"net/http"
	"strings"
	"time"
)

var NEWLINEBYTES = []byte("\n")

func WalkBulkRequests(data []byte,eachLineFunc func(eachLine []byte) (skipNextLine bool), metaFunc func(metaBytes []byte, actionStr, index, typeName, id,routing string,offset int) (err error), payloadFunc func(payloadBytes []byte, actionStr, index, typeName, id,routing string)) (int, error) {

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
			actionStr, index, typeName, id,routing,err = ParseActionMeta(line)
			//log.Error(docCount,",",actionStr, index, typeName, id,routing,err,",",string(line))
			if err!=nil{
				panic(err)
			}

			err = metaFunc(line, actionStr, index, typeName, id,routing,docCount)
			if err != nil {
				panic(err)
			}

			docCount++

			if actionStr == ActionDelete {
				nextIsMeta = true
			}
		} else {
			nextIsMeta = true
			payloadFunc(line, actionStr, index, typeName, id,routing)
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

	Compress                 bool   `config:"compress"`
	RetryDelayInSeconds     int    `config:"retry_delay_in_seconds"`
	RejectDelayInSeconds    int    `config:"reject_retry_delay_in_seconds"`
	MaxRejectRetryTimes     int    `config:"max_reject_retry_times"`
	MaxRetryTimes           int    `config:"max_retry_times"`
	RequestTimeoutInSecond  int    `config:"request_timeout_in_second"`

	InvalidRequestsQueue    string `config:"invalid_queue"`
	DeadletterRequestsQueue string `config:"dead_letter_queue"`



	BulkResponseParseConfig BulkResponseParseConfig `config:"response_handle"`

	RemoveDuplicatedNewlines bool   `config:"remove_duplicated_newlines"`
}


type BulkResponseParseConfig struct {

	SaveSuccessBulkResultToMessageQueue    bool `config:"save_success_results"`
	SaveErrorBulkResultToMessageQueue      bool `config:"save_error_results"`
	OutputBulkResults      				   bool `config:"output_bulk_stats"`

	IncludeIndexStats      				   bool `config:"include_index_stats"`
	IncludeOperationStats      			   bool `config:"include_operation_stats"`


	IncludeErrorDetails        bool `config:"include_error_details"`
	MaxItemOfErrorDetailsCount int  `config:"max_error_details_count"`

	SaveBusyBulkResultToMessageQueue       bool `config:"save_busy_results"`
	BulkResultMessageQueue                 string `config:"bulk_result_message_queue"`
	BulkResultMessageMaxRequestBodyLength  int    `config:"max_request_body_size"`
	BulkResultMessageMaxResponseBodyLength int    `config:"max_response_body_size"`

	RetryException RetryException `config:"retry_exception"`
}


type RetryException struct {
	Retry429 			bool `config:"retry_429"`
	ResponseCode       []int 	`config:"code"`
	ResponseKeyword    []string `config:"keyword"`
}

func (this *RetryException) ShouldSkipRetry(code int,msg string)bool{

	if len(this.ResponseCode)>0{
		for _,v:=range this.ResponseCode{
			if v==code{
				return true
			}
		}
	}

	if len(this.ResponseKeyword)>0 && len(msg)>0{
		for _,v:=range this.ResponseKeyword{
			if util.ContainStr(msg,v){
				log.Debug("message: ",msg," contains keyword: [",v, "], skip retry")
				return true
			}
		}
	}

	return false
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
	BulkMaxDocsCount:        1000,
	BulkSizeInMb:            10,
	Compress:                false,
	RetryDelayInSeconds:     1,
	RejectDelayInSeconds:    1,
	MaxRejectRetryTimes:     60,
	MaxRetryTimes:           10,


	DeadletterRequestsQueue: "bulk_dead_requests",
	BulkResponseParseConfig: BulkResponseParseConfig{
		BulkResultMessageMaxRequestBodyLength:  10*1024,
		BulkResultMessageMaxResponseBodyLength: 10*1024,

		BulkResultMessageQueue:  "bulk_result_messages",

		SaveSuccessBulkResultToMessageQueue:  true,
		SaveErrorBulkResultToMessageQueue:true,
		SaveBusyBulkResultToMessageQueue:true,
		OutputBulkResults:false,

		IncludeErrorDetails:        true,
		IncludeIndexStats:          true,
		IncludeOperationStats:      true,
		MaxItemOfErrorDetailsCount: 50,

		RetryException: RetryException{Retry429: true},
	},
	RequestTimeoutInSecond:  60,
}

type BulkProcessor struct {
	Config BulkProcessorConfig
}

func (joint *BulkProcessor) Bulk(tag string, metadata *ElasticsearchMetadata, host string, buffer *BulkBuffer) (continueNext bool, statsRet map[int]int, err error) {

	statsRet = make(map[int]int)

	if buffer == nil || buffer.GetMessageSize() == 0 {
		return true,statsRet, errors.New("invalid bulk requests, message is nil")
	}

	host = metadata.GetActivePreferredHost(host)

	if host==""{
		panic("invalid host")
	}

	httpClient := metadata.GetHttpClient(host)

	var url string
	if metadata.IsTLS() {
		url = fmt.Sprintf("https://%s/_bulk", host)
	} else {
		url = fmt.Sprintf("http://%s/_bulk", host)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)   // <- do not forget to release
	defer fasthttp.ReleaseResponse(resp) // <- do not forget to release

	req.SetRequestURI(url)

	req.Header.SetMethod(http.MethodPost)
	req.Header.SetUserAgent("_bulk")
	req.Header.SetContentType("application/x-ndjson")

	if metadata.Config.BasicAuth != nil {
		req.URI().SetUsername(metadata.Config.BasicAuth.Username)
		req.URI().SetPassword(metadata.Config.BasicAuth.Password)
	}

	//acceptGzipped := req.AcceptGzippedResponse()
	//compressed := false

	// handle last \n
	buffer.SafetyEndWithNewline()
	data := buffer.GetMessageBytes()

	if joint.Config.RemoveDuplicatedNewlines{
		//handle double \n
		data=bytes.ReplaceAll(data,[]byte("\n\n"),[]byte("\n"))
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
	orignalSchema := string(req.URI().Scheme())
	orignalHost := string(req.URI().Host())

	if host!=""&&req.Host()==nil||string(req.Host())!=orignalHost{
		req.Header.SetHost(host)
	}

	if metadata.GetSchema() != orignalSchema {
		req.URI().SetScheme(metadata.GetSchema())
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
	
	//execute
	err = httpClient.DoTimeout(req, resp, time.Duration(joint.Config.RequestTimeoutInSecond)*time.Second)

	if err != nil {
		if rate.GetRateLimiter(metadata.Config.ID, host+"5xx_on_error", 1, 1, 5*time.Second).Allow() {
			log.Error("status:", resp.StatusCode(), ",", host, ",", err, " ", util.SubString(util.UnsafeBytesToString(resp.GetRawBody()), 0, 256))
			time.Sleep(2 * time.Second)
		}
		return false,statsRet, err
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

	//restore schema
	req.URI().SetScheme(orignalSchema)
	req.SetHost(orignalHost)

	if resp == nil {
		if global.Env().IsDebug {
			log.Error(err)
		}
		return false,statsRet, err
	}
	
	// Do we need to decompress the response?
	var resbody = resp.GetRawBody()
	
	if global.Env().IsDebug {
		log.Trace(resp.StatusCode(), util.UnsafeBytesToString(util.EscapeNewLine(data)), util.UnsafeBytesToString(util.EscapeNewLine(resbody)))
	}

	if retryTimes>0{
		log.Errorf("#%v, code:%v",retryTimes,resp.StatusCode())
	}

	//for response
	labels:=util.MapStr{
		"cluster_id":       metadata.Config.ID,
		"host":       orignalHost,
		"queue":     buffer.Queue,
	}
	if retryTimes > 0 {
		labels["retry_times"] = retryTimes
	}

	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusCreated {
		
		//如果是部分失败，应该将可以重试的做完，然后记录失败的消息再返回不继续
		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {

			containError, statsCodeStats,_:= HandleBulkResponse(req, resp,labels, data, resbody,successItems, nonRetryableItems, retryableItems,joint.Config.BulkResponseParseConfig)

			for k,v:=range statsCodeStats{
				stats.IncrementBy("bulk::"+tag,util.ToString(k), int64(v))
				statsRet[k] = statsRet[k] + v
			}

			if retryTimes>0||global.Env().IsDebug{
				log.Infof("#%v, code:%v, contain_err:%v, status:%v, success:%v, failure:%v, invalid:%v",
					retryTimes,resp.StatusCode(),containError,statsCodeStats,successItems.GetMessageCount(),retryableItems.GetMessageCount(),nonRetryableItems.GetMessageCount())
			}

			if containError{
					count:=retryableItems.GetMessageCount()
					if count > 0 {
						log.Debugf("%v, retry item: %v",tag,count)
						retryableItems.SafetyEndWithNewline()
						bodyBytes:=retryableItems.GetMessageBytes()
						req.SetRawBody(bodyBytes)
						delayTime := joint.Config.RejectDelayInSeconds

						if delayTime <= 0 {
							delayTime = 5
						}

						time.Sleep(time.Duration(delayTime) * time.Second)

						if joint.Config.MaxRejectRetryTimes <= 0 {
							joint.Config.MaxRejectRetryTimes = 12 //1min
						}

						if retryTimes >= joint.Config.MaxRejectRetryTimes {

							//continue retry before is back
							if !metadata.IsAvailable() {
								return false,statsRet, errors.Errorf("elasticsearch [%v] is not available", metadata.Config.Name)
							}

							data := req.OverrideBodyEncode(bodyBytes, true)
							queue.Push(queue.GetOrInitConfig(metadata.Config.ID+"_dead_letter_queue"), data)
							return true,statsRet, errors.Errorf("bulk partial failure, retried %v times, quit retry", retryTimes)
						}
						log.Infof("%v, bulk partial failure, #%v retry, %v items left, size: %v", tag,retryTimes,retryableItems.GetMessageCount(),retryableItems.GetMessageSize())
						retryTimes++
						stats.Increment("elasticsearch."+tag+"."+metadata.Config.Name+".bulk", "retry")
						goto DO
					}

					var continueNext =false
					//skip all failure messages
					if nonRetryableItems.GetMessageCount() > 0 && retryableItems.GetMessageCount() == 0 {
						////handle 400 error
						if joint.Config.InvalidRequestsQueue != "" {
							queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
						}
						continueNext=true
					}else if nonRetryableItems.GetMessageCount() == 0 && retryableItems.GetMessageCount() == 0{
						//no retryable
						continueNext=true
					}
					if retryableItems.GetMessageCount()>0{
						continueNext=false
					}

					return continueNext,statsRet, errors.Errorf("bulk response contains error, %v, %v, %v, none-retryable:%v, retryable:%v",metadata.Config.Name, statsCodeStats,statsRet,nonRetryableItems.GetMessageCount(),retryableItems.GetMessageCount())
				}
		}
		return true,statsRet, nil
	}else{

		statsRet[resp.StatusCode()] = statsRet[resp.StatusCode()] + buffer.GetMessageCount()

		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {
			HandleBulkResponse(req, resp, labels, data, resbody, successItems, nonRetryableItems, retryableItems, joint.Config.BulkResponseParseConfig)
		}

		if resp.StatusCode() == 429 {
			return false,statsRet, errors.Errorf("code 429, [%v] is too busy", metadata.Config.Name)
		} else if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			////handle 400 error
			if joint.Config.InvalidRequestsQueue != "" {
				queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
			}
			return true,statsRet, errors.Errorf("invalid requests, code: %v", resp.StatusCode())
		} else {

			if global.Env().IsDebug {
				log.Debugf("status:", resp.StatusCode(), ",request:", util.UnsafeBytesToString(req.GetRawBody()), ",response:", util.UnsafeBytesToString(resbody))
			}

			if joint.Config.BulkResponseParseConfig.RetryException.ShouldSkipRetry(resp.StatusCode(),util.UnsafeBytesToString(resp.GetRawBody())){
				truncatedResponse:=util.SubString(util.UnsafeBytesToString(resbody), 0, joint.Config.BulkResponseParseConfig.BulkResultMessageMaxRequestBodyLength)
				log.Warnf("code: %v, but hit condition to skip retry, response: %v",resp.StatusCode(),truncatedResponse)
				return true,statsRet, errors.Errorf("code: %v, response: %v", resp.StatusCode(),truncatedResponse)
			}

			return false,statsRet, errors.Errorf("bulk requests failed, code: %v", resp.StatusCode())
		}
	}
}


func HandleBulkResponse(req  *fasthttp.Request, resp *fasthttp.Response ,tag util.MapStr, requestBytes, resbody []byte,successItems *BulkBuffer, nonRetryableItems, retryableItems *BulkBuffer,options BulkResponseParseConfig) (bool, map[int]int,util.MapStr) {
	nonRetryableItems.ResetData()
	retryableItems.ResetData()
	successItems.ResetData()

	containError := util.LimitedBytesSearch(resbody, []byte("\"errors\":true"), 64)
	var statsCodeStats = map[int]int{}
	invalidDocStatus := map[int]int{}
	invalidDocError := map[int]string{}
	invalidDocID := map[int]string{}

	var indexStatsData map[string]int= map[string]int{}
	var actionStatsData map[string]int= map[string]int{}

	items, _, _, err := jsonparser.Get(resbody, "items")
	if err == nil {
		var docOffset=0
		jsonparser.ArrayEach(items, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			item,_,_,err:=jsonparser.Get(value,"index")
			if err!=nil{
				item,_,_,err=jsonparser.Get(value,"delete")
				if err!=nil{
					item,_,_,err=jsonparser.Get(value,"create")
					if err!=nil{
						item,_,_,err=jsonparser.Get(value,"update")
					}
				}
			}
			if err==nil{

				//id can be nil
				docId,err:=jsonparser.GetString(item,"_id")
				if err == nil {
					docId=strings.Clone(docId)
				}

				code1,err:=jsonparser.GetInt(item,"status")
				if err != nil {
					panic(err)
				}
				code:=int(code1)
				x, ok := statsCodeStats[code]
				if !ok {
					x = 0
				}
				x++
				statsCodeStats[code] = x
				erObj,_,_,err:=jsonparser.Get(item,"error")
				if err==nil&&erObj != nil {
					invalidDocID[docOffset] = docId
					invalidDocStatus[docOffset] = code
					invalidDocError[docOffset] = util.UnsafeBytesToString(erObj)
				}
			}
			docOffset++
		})

		if global.Env().IsDebug {
			log.Debug(tag, " bulk invalid, status:", statsCodeStats, ",invalid status:", invalidDocStatus)
		}

		var match = false
		var retryable = false
		WalkBulkRequests(requestBytes,func(eachLine []byte) (skipNextLine bool) {
			return false
		}, func(metaBytes []byte, actionStr, index, typeName, id,routing string,offset int) (err error) {

			if options.IncludeIndexStats {
				//init
				if indexStatsData == nil {
					if indexStatsData == nil {
						indexStatsData = map[string]int{}
					}
				}

				//stats
				v, ok := indexStatsData[index]
				if !ok {
					indexStatsData[index] = 1
				} else {
					indexStatsData[index] = v + 1
				}
			}

			if options.IncludeOperationStats {
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
			if id==""{
				id=fmt.Sprintf("N/A(%v)",offset)
			}
			if match {

				docID:=invalidDocID[offset]
				if docID==""{
					docID=fmt.Sprintf("N/A(%v)",offset)
				}
				//log.Error(offset,",",id,",",docID,",",code,",",match)

				if code==429 && options.RetryException.Retry429{
					retryable = true
				}else if code >= 400 && code < 500{ //find invalid request 409
					retryable = false
				}else{
					retryable = true
				}

				//last check
				errMsg:=invalidDocError[offset]
				if retryable&&options.RetryException.ShouldSkipRetry(code,errMsg){
					retryable = false
				}

				if global.Env().IsDebug{
					log.Debugf("index:%v, offset:%v, docID:%v, action:%v, code:%v, retryable:%v, reason:%v",index,offset,docID,actionStr,code,retryable,errMsg)
				}

				if retryable{
					retryableItems.WriteNewByteBufferLine("retryable",metaBytes)
					retryableItems.WriteMessageID(docID)
					retryableItems.WriteErrorReason(errMsg)
				}else{
					nonRetryableItems.WriteNewByteBufferLine("none-retryable",metaBytes)
					nonRetryableItems.WriteMessageID(docID)
					nonRetryableItems.WriteErrorReason(errMsg)
				}
			}else{
				successItems.WriteNewByteBufferLine("success",metaBytes)
				successItems.WriteMessageID(id)
			}
			return nil
		}, func(payloadBytes []byte, actionStr, index, typeName, id,routing string) {
			if match {
				if payloadBytes != nil && len(payloadBytes) > 0 {
					if retryable {
						retryableItems.WriteNewByteBufferLine("retryable",payloadBytes)
					} else {
						nonRetryableItems.WriteNewByteBufferLine("none-retryable",payloadBytes)
					}
				}
			}else{
				if payloadBytes != nil && len(payloadBytes) > 0 {
					successItems.WriteNewByteBufferLine("success", payloadBytes)
				}
			}
		})
	}

	//save log and stats
	var bulkResult util.MapStr

	//skip 429 response
	if resp.StatusCode()==429&&!options.SaveBusyBulkResultToMessageQueue {
		return containError, statsCodeStats,bulkResult
	}

	if options.OutputBulkResults||options.SaveErrorBulkResultToMessageQueue || options.SaveSuccessBulkResultToMessageQueue {

		bulkResult = util.MapStr{
			"error": containError,
			"codes":  util.GetIntMapKeys(statsCodeStats),
			"indices":util.GetStringIntMapKeys(indexStatsData),
			"actions":util.GetStringIntMapKeys(actionStatsData),
			"summary":util.MapStr{
				"failure" : util.MapStr{
					"count": retryableItems.GetMessageCount(),
					"size":  retryableItems.GetMessageSize(),
				},
				"invalid" : util.MapStr{
					"count": nonRetryableItems.GetMessageCount(),
					"size":  nonRetryableItems.GetMessageSize(),
				},
				"success" : util.MapStr{
					"count": successItems.GetMessageCount(),
					"size":  successItems.GetMessageSize(),
				},
			},
			"stats": util.MapStr{
				"code":statsCodeStats,
				"indices":indexStatsData,
				"actions":actionStatsData,
			},
		}


		if options.IncludeErrorDetails {
			detail:=util.MapStr{}
			if nonRetryableItems.GetMessageCount() > 0 {
				ids := nonRetryableItems.MessageIDs
				reasons := nonRetryableItems.Reason
				if nonRetryableItems.GetMessageCount() > options.MaxItemOfErrorDetailsCount {
					ids = ids[0:options.MaxItemOfErrorDetailsCount]
					reasons = reasons[0:options.MaxItemOfErrorDetailsCount]
				}
				detail["invalid"] = util.MapStr{
					"documents": ids,
					"reasons":    reasons,
				}
			}

			if retryableItems.GetMessageCount() > 0 {
				ids := retryableItems.MessageIDs
				reasons := retryableItems.Reason
				if retryableItems.GetMessageCount() > options.MaxItemOfErrorDetailsCount {
					ids = ids[0:options.MaxItemOfErrorDetailsCount]
					reasons = reasons[0:options.MaxItemOfErrorDetailsCount]
				}
				detail["failure"] = util.MapStr{
					"documents": ids,
					"reasons":    reasons,
				}
			}

			if len(detail)>0{
				bulkResult["detail"]=detail
			}
		}

		if options.BulkResultMessageQueue != "" {
			//save message bytes, with metadata, set codec to wrapped bulk messages
			queue.Push(queue.GetOrInitConfig(options.BulkResultMessageQueue), util.MustToJSONBytes(util.MapStr{
				"timestamp": time.Now(),
				"bulk_results":   bulkResult,
				"labels":   tag,
				"node":      global.Env().SystemConfig.NodeConfig,
				"request": util.MapStr{
					"uri":  req.URI().String(),
					"body_length": len(requestBytes),
					"body": util.SubString(util.UnsafeBytesToString(req.GetRawBody()), 0, options.BulkResultMessageMaxRequestBodyLength),
				},
				"response": util.MapStr{
					"status_code": resp.StatusCode(),
					"body_length": len(resbody),
					"body":        util.SubString(util.UnsafeBytesToString(resbody), 0, options.BulkResultMessageMaxResponseBodyLength),
				},
			}))
		}
	}

	return containError, statsCodeStats,bulkResult
}
