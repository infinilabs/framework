package pprofhandler

import (
	fasthttp2 "infini.sh/framework/lib/fasthttp"
	fasthttpadaptor2 "infini.sh/framework/lib/fasthttp/fasthttpadaptor"
	"net/http/pprof"
	rtp "runtime/pprof"
	"strings"
)

var (
	cmdline = fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Cmdline)
	profile = fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Profile)
	symbol  = fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Symbol)
	trace   = fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Trace)
	index   = fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Index)
)

// PprofHandler serves server runtime profiling data in the format expected by the pprof visualization tool.
//
// See https://golang.org/pkg/net/http/pprof/ for details.
func PprofHandler(ctx *fasthttp2.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "text/html")
	if strings.HasPrefix(string(ctx.Path()), "/debug/pprof/cmdline") {
		cmdline(ctx)
	} else if strings.HasPrefix(string(ctx.Path()), "/debug/pprof/profile") {
		profile(ctx)
	} else if strings.HasPrefix(string(ctx.Path()), "/debug/pprof/symbol") {
		symbol(ctx)
	} else if strings.HasPrefix(string(ctx.Path()), "/debug/pprof/trace") {
		trace(ctx)
	} else {
		for _, v := range rtp.Profiles() {
			ppName := v.Name()
			if strings.HasPrefix(string(ctx.Path()), "/debug/pprof/"+ppName) {
				namedHandler := fasthttpadaptor2.NewFastHTTPHandlerFunc(pprof.Handler(ppName).ServeHTTP)
				namedHandler(ctx)
				return
			}
		}
		index(ctx)
	}
}
