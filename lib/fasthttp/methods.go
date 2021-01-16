package fasthttp

// HTTP methods were copied from net/http.
const (
	MethodGet     = "GET"     // RFC 7231, 4.3.1
	MethodHead    = "HEAD"    // RFC 7231, 4.3.2
	MethodPost    = "POST"    // RFC 7231, 4.3.3
	MethodPut     = "PUT"     // RFC 7231, 4.3.4
	MethodPatch   = "PATCH"   // RFC 5789
	MethodDelete  = "DELETE"  // RFC 7231, 4.3.5
	MethodConnect = "CONNECT" // RFC 7231, 4.3.6
	MethodOptions = "OPTIONS" // RFC 7231, 4.3.7
	MethodTrace   = "TRACE"   // RFC 7231, 4.3.8
)
var (
	MethodGetBytes     = []byte("GET")     // RFC 7231, 4.3.1
	MethodHeadBytes    = []byte("HEAD" )    // RFC 7231, 4.3.2
	MethodPostBytes    = []byte("POST")     // RFC 7231, 4.3.3
	MethodPutBytes     = []byte("PUT" )     // RFC 7231, 4.3.4
	MethodPatchBytes   = []byte("PATCH" )   // RFC 5789
	MethodDeleteBytes  = []byte("DELETE")   // RFC 7231, 4.3.5
	MethodConnectBytes = []byte("CONNECT")  // RFC 7231, 4.3.6
	MethodOptionsBytes = []byte("OPTIONS")  // RFC 7231, 4.3.7
	MethodTraceBytes   = []byte("TRACE" )   // RFC 7231, 4.3.8
)