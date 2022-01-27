package api

import (
	"fmt"
	"infini.sh/framework/core/util"
	"net/http"
	"testing"
	"time"
)

func TestGetMetricParams(t *testing.T) {
	handler:=APIHandler{}
	req:=http.Request{}
	bucketSize, min, max, err:=handler.getMetricRangeAndBucketSize(&req,60,15)

	fmt.Println(bucketSize)
	fmt.Println(util.FormatUnixTimestamp(min/1000))//2022-01-27 15:28:57
	fmt.Println(util.FormatUnixTimestamp(max/1000))//2022-01-27 15:28:57
	fmt.Println(time.Now())//2022-01-27 15:28:57

	fmt.Println(bucketSize, min, max, err)
}
