// Package expvarhandler provides fasthttp-compatible request handler
// serving expvars.
package expvarhandler

import (
	"expvar"
	"fmt"
	fasthttp2 "infini.sh/framework/lib/fasthttp"
	"regexp"
)

var (
	expvarHandlerCalls = expvar.NewInt("expvarHandlerCalls")
	expvarRegexpErrors = expvar.NewInt("expvarRegexpErrors")

	defaultRE = regexp.MustCompile(".")
)

// ExpvarHandler dumps json representation of expvars to http response.
//
// Expvars may be filtered by regexp provided via 'r' query argument.
//
// See https://golang.org/pkg/expvar/ for details.
func ExpvarHandler(ctx *fasthttp2.RequestCtx) {
	expvarHandlerCalls.Add(1)

	ctx.Response.Reset()

	r, err := getExpvarRegexp(ctx)
	if err != nil {
		expvarRegexpErrors.Add(1)
		fmt.Fprintf(ctx, "Error when obtaining expvar regexp: %s", err)
		ctx.SetStatusCode(fasthttp2.StatusBadRequest)
		return
	}

	fmt.Fprintf(ctx, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if r.MatchString(kv.Key) {
			if !first {
				fmt.Fprintf(ctx, ",\n")
			}
			first = false
			fmt.Fprintf(ctx, "\t%q: %s", kv.Key, kv.Value)
		}
	})
	fmt.Fprintf(ctx, "\n}\n")

	ctx.SetContentType("application/json; charset=utf-8")
}

func getExpvarRegexp(ctx *fasthttp2.RequestCtx) (*regexp.Regexp, error) {
	r := string(ctx.QueryArgs().Peek("r"))
	if len(r) == 0 {
		return defaultRE, nil
	}
	rr, err := regexp.Compile(r)
	if err != nil {
		return nil, fmt.Errorf("cannot parse r=%q: %s", r, err)
	}
	return rr, nil
}
