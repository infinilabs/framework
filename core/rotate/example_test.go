package rotate

import (
	"log"
)

// To use lumberjack with the standard library's log package, just pass it into
// the SetOutput function when your application starts.
func Example() {
	log.SetOutput(&RotateWriter{
		Filename:         "/var/log/myapp/foo.log",
		MaxFileSize:      500, // megabytes
		MaxRotationCount: 3,
		MaxFileAge:       28,   // days
		Compress:         true, // disabled by default
	})
}
