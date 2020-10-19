// +build gofuzz

package request

import (
	"bufio"
	"bytes"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
)

func Fuzz(data []byte) int {
	req := fasthttp2.AcquireRequest()
	defer fasthttp2.ReleaseRequest(req)

	if err := req.ReadLimitBody(bufio.NewReader(bytes.NewReader(data)), 1024*1024); err != nil {
		return 0
	}

	w := bytes.Buffer{}
	if _, err := req.WriteTo(&w); err != nil {
		return 0
	}

	return 1
}
