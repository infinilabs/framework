//go:build windows
// +build windows

package fasthttp

import "testing"

func TestURIPathNormalizeIssue86(t *testing.T) {
	t.Parallel()

	// see https://github.com/rubyniu105/framework/lib/fasthttp/issues/86
	var u URI

	testURIPathNormalize(t, &u, `C:\a\b\c\fs.go`, `C:\a\b\c\fs.go`)
}
