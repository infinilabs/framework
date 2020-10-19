// +build gofuzz

package response

import (
	"bufio"
	"bytes"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
)

func Fuzz(data []byte) int {
	res := fasthttp2.AcquireResponse()
	defer fasthttp2.ReleaseResponse(res)

	if err := res.ReadLimitBody(bufio.NewReader(bytes.NewReader(data)), 1024*1024); err != nil {
		return 0
	}

	w := bytes.Buffer{}
	if _, err := res.WriteTo(&w); err != nil {
		return 0
	}

	return 1
}
