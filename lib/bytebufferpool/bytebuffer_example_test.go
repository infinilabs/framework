package bytebufferpool_test

import (
	"fmt"

	"github.com/rubyniu105/framework/lib/bytebufferpool"
)

func ExampleByteBuffer() {
	bb := bytebufferpool.Get("test")

	bb.WriteString("first line\n")
	bb.Write([]byte("second line\n"))
	bb.B = append(bb.B, "third line\n"...)

	fmt.Printf("bytebuffer contents=%q", bb.B)

	// It is safe to release byte buffer now, since it is
	// no longer used.
	bytebufferpool.Put("test", bb)
}
