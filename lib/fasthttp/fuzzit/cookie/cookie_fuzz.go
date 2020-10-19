// +build gofuzz

package cookie

import (
	"bytes"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
)

func Fuzz(data []byte) int {
	c := fasthttp2.AcquireCookie()
	defer fasthttp2.ReleaseCookie(c)

	if err := c.ParseBytes(data); err != nil {
		return 0
	}

	w := bytes.Buffer{}
	if _, err := c.WriteTo(&w); err != nil {
		return 0
	}

	return 1
}
