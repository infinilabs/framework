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

func WalkBulkRequests(data []byte,eachLineFunc func(eachLine []byte) (skipNextLine bool), metaFunc func(metaBytes []byte, actionStr, index, typeName, id,routing string) (err error), payloadFunc func(payloadBytes []byte, actionStr, index, typeName, id,routing string)) (int, error) {

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
			if err!=nil{
				panic(err)
			}

			err = metaFunc(line, actionStr, index, typeName, id,routing)
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

	SaveSuccessBulkResultToMessageQueue    bool `config:"save_success_results"`
	SaveBusyBulkResultToMessageQueue       bool `config:"save_busy_results"`
	BulkResultMessageQueue                 string `config:"bulk_result_message_queue"`
	BulkResultMessageMaxRequestBodyLength  int    `config:"max_request_body_size"`
	BulkResultMessageMaxResponseBodyLength int    `config:"max_response_body_size"`

	RetryException RetryException `config:"retry_exception"`

	RemoveDuplicatedNewlines bool   `config:"remove_duplicated_newlines"`
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

	BulkResultMessageMaxRequestBodyLength:  10*1024,
	BulkResultMessageMaxResponseBodyLength: 10*1024,

	BulkResultMessageQueue:  "bulk_result_messages",
	DeadletterRequestsQueue: "bulk_dead_requests",
	RetryException: RetryException{Retry429: true},
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
	//req.URI().Update(url)

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
		data=util.ReplaceByte(data,[]byte("\n\n"),[]byte("\n"))
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

	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusCreated {
		
		//如果是部分失败，应该将可以重试的做完，然后记录失败的消息再返回不继续
		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {

			containError, statsCodeStats := HandleBulkResponse(tag, data, resbody,successItems, nonRetryableItems, retryableItems,joint.Config.RetryException)

			for k,v:=range statsCodeStats{
				stats.IncrementBy("bulk::"+tag,util.ToString(k), int64(v))
				statsRet[k] = statsRet[k] + v
			}

			if retryTimes>0{
				log.Errorf("#%v, code:%v, contain_err:%v, status:%v",retryTimes,resp.StatusCode(),containError,statsCodeStats)
			}

			if containError || joint.Config.SaveSuccessBulkResultToMessageQueue {

				if joint.Config.BulkResultMessageQueue !=""{

						elasticMap:=util.MapStr{
							"cluster_id": metadata.Config.ID,
							"error": containError,
							"bulk_stats.code":util.GetIntMapKeys(statsCodeStats),
							"bulk_stats.stats":statsCodeStats,
						}
						if retryTimes>0{
							elasticMap["retry_times"]=retryTimes
						}

						//save message bytes, with metadata, set codec to wrapped bulk messages
						queue.Push(queue.GetOrInitConfig(joint.Config.BulkResultMessageQueue), util.MustToJSONBytes(util.MapStr{
							"timestamp": time.Now(),
							"elastic":elasticMap,
							"queue":      buffer.Queue,
							"request": util.MapStr{
								"uri":  req.URI().String(),
								"body": util.SubString(util.UnsafeBytesToString(req.GetRawBody()), 0, joint.Config.BulkResultMessageMaxRequestBodyLength),
							},
							"response": util.MapStr{
								"status_code": resp.StatusCode(),
								"body":   util.SubString(util.UnsafeBytesToString(resbody), 0, joint.Config.BulkResultMessageMaxResponseBodyLength),
							},
						}))
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

					//skip all failure messages
					if nonRetryableItems.GetMessageCount() > 0 && retryableItems.GetMessageCount() == 0 {
						////handle 400 error
						if joint.Config.InvalidRequestsQueue != "" {
							queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
						}
						return true,statsRet, errors.Errorf("[%v] invalid bulk requests", metadata.Config.Name)
					}
					return false,statsRet, errors.Errorf("bulk response contains error, %v, %v", statsCodeStats,statsRet)
				}
			}
		}
		return true,statsRet, nil
	}else{

		statsRet[resp.StatusCode()] = statsRet[resp.StatusCode()] + buffer.GetMessageCount()

		if joint.Config.BulkResultMessageQueue !="" && !(resp.StatusCode()==429&&!joint.Config.SaveBusyBulkResultToMessageQueue){
			//save message bytes, with metadata, set codec to wrapped bulk messages
			queue.Push(queue.GetOrInitConfig(joint.Config.BulkResultMessageQueue), util.MustToJSONBytes(util.MapStr{
				"timestamp": time.Now(),
				"queue":      buffer.Queue,
				"request": util.MapStr{
					"uri":  req.URI().String(),
					"body": util.SubString(util.UnsafeBytesToString(req.GetRawBody()), 0, joint.Config.BulkResultMessageMaxRequestBodyLength),
				},
				"elastic":util.MapStr{
					"cluster_id": metadata.Config.ID,
					"bulk_stats.code":util.GetIntMapKeys(statsRet),
					"bulk_stats.stats":statsRet,
				},
				"response": util.MapStr{
					"status_code": resp.StatusCode(),
					"body":   util.SubString(util.UnsafeBytesToString(resbody), 0, joint.Config.BulkResultMessageMaxResponseBodyLength),
				},
			}))
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

			if joint.Config.RetryException.ShouldSkipRetry(resp.StatusCode(),util.UnsafeBytesToString(resp.GetRawBody())){
				truncatedResponse:=util.SubString(util.UnsafeBytesToString(resbody), 0, joint.Config.BulkResultMessageMaxRequestBodyLength)
				log.Warnf("code: %v, but hit condition to skip retry, response: %v",resp.StatusCode(),truncatedResponse)
				return true,statsRet, errors.Errorf("code: %v, response: %v", resp.StatusCode(),truncatedResponse)
			}

			return false,statsRet, errors.Errorf("bulk requests failed, code: %v", resp.StatusCode())
		}
	}
}

func HandleBulkResponse(tag string, requestBytes, resbody []byte,successItems *BulkBuffer, nonRetryableItems, retryableItems *BulkBuffer,retryException RetryException) (bool, map[int]int) {
	nonRetryableItems.Reset()
	retryableItems.Reset()
	successItems.Reset()

	containError := util.LimitedBytesSearch(resbody, []byte("\"errors\":true"), 64)
	var statsCodeStats = map[int]int{}
	invalidDocStatus := map[string]int{}
	invalidDocError := map[string]string{}

	items, _, _, err := jsonparser.Get(resbody, "items")
	if err != nil {
		panic(err)
	}

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
			docId,err:=jsonparser.GetString(item,"_id")
			if err != nil {
				panic(err)
			}
			docId=strings.Clone(docId)
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
				invalidDocStatus[docId] = code
				invalidDocError[docId] = util.UnsafeBytesToString(erObj)
			}
		}
	})

	if len(invalidDocStatus) > 0 {
		if global.Env().IsDebug {
			log.Debug(tag," bulk invalid, status:", statsCodeStats)
		}
	}

	var match = false
	var retryable = false
	WalkBulkRequests(requestBytes,func(eachLine []byte) (skipNextLine bool) {
		return false
	}, func(metaBytes []byte, actionStr, index, typeName, id,routing string) (err error) {

		var code int
		code, match = invalidDocStatus[id]
		if match {
			if code==429 && retryException.Retry429{
				retryable = true
			}else if code >= 400 && code < 500{ //find invalid request 409
				retryable = false
			} else {

				errMsg:=invalidDocError[id]

				if retryException.ShouldSkipRetry(code,errMsg){
					retryable = false
				}else{
					retryable = true
				}
			}

			if retryable{
				retryableItems.WriteNewByteBufferLine("retryable",metaBytes)
				retryableItems.WriteMessageID(id)
			}else{
				nonRetryableItems.WriteNewByteBufferLine("none-retryable",metaBytes)
				nonRetryableItems.WriteMessageID(id)
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

	return containError, statsCodeStats
}
