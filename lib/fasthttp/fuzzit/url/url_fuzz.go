// +build gofuzz

package url

import (
	"bytes"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
)

func Fuzz(data []byte) int {
	u := fasthttp2.AcquireURI()
	defer fasthttp2.ReleaseURI(u)

	u.UpdateBytes(data)

	w := bytes.Buffer{}
	if _, err := u.WriteTo(&w); err != nil {
		return 0
	}

	return 1
}
