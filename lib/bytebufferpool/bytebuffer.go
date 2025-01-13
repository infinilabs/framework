package bytebufferpool

import (
	"io"
	"math"
	"time"
)

// ByteBuffer provides byte buffer, which can be used for minimizing
// memory allocations.
//
// ByteBuffer may be used with functions appending data to the given []byte
// slice. See example code for details.
//
// Use Get for obtaining an empty byte buffer.
type ByteBuffer struct {
	LastAccess time.Time
	Used       uint32
	ID         uint32
	// B is a byte buffer to use in append-like workloads.
	// See example code for details.
	B   []byte
	cap int
}

// Len returns the size of the byte buffer.
func (b *ByteBuffer) Len() int {
	return len(b.B)
}

func (b *ByteBuffer) Cap() int {
	if b.cap > 0 {
		return b.cap
	}
	return cap(b.B)
}

// ReadFrom implements io.ReaderFrom.
//
// The function appends all the data read from r to b.
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	p := b.B
	nStart := int64(len(p))
	nMax := int64(cap(p))
	n := nStart
	if nMax == 0 {
		nMax = 64
		p = make([]byte, nMax)
	} else {
		p = p[:nMax]
	}
	for {
		if n == nMax {
			nMax *= 2
			bNew := make([]byte, nMax)
			copy(bNew, p)
			p = bNew
		}
		nn, err := r.Read(p[n:])
		n += int64(nn)
		if err != nil {
			b.B = p[:n]
			n -= nStart
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// WriteTo implements io.WriterTo.
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b.B)
	b.cap = -1
	return int64(n), err
}

// Bytes returns b.B, i.e. all the bytes accumulated in the buffer.
//
// The purpose of this function is bytes.Buffer compatibility.
func (b *ByteBuffer) Bytes() []byte {
	return b.B
}

func (b *ByteBuffer) WriteBytesArray(ps ...[]byte) (count int, err error) {
	for _, p := range ps {
		v, err := b.Write(p)
		count += v
		if err != nil {
			break
		}
	}
	b.cap = -1
	return count, err
}

// Write implements io.Writer - it appends p to ByteBuffer.B
func (b *ByteBuffer) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	b.cap = -1
	return len(p), nil
}

// WriteByte appends the byte c to the buffer.
//
// The purpose of this function is bytes.Buffer compatibility.
//
// The function always returns nil.
func (b *ByteBuffer) WriteByte(c byte) error {
	b.B = append(b.B, c)
	b.cap = -1
	return nil
}

// WriteString appends s to ByteBuffer.B.
func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.B = append(b.B, s...)
	b.cap = -1
	return len(s), nil
}

// Set sets ByteBuffer.B to p.
func (b *ByteBuffer) Set(p []byte) {
	b.B = append(b.B[:0], p...)
	b.cap = -1
}

// SetString sets ByteBuffer.B to s.
func (b *ByteBuffer) SetString(s string) {
	b.B = append(b.B[:0], s...)
	b.cap = -1
}

// String returns string representation of ByteBuffer.B.
func (b *ByteBuffer) String() string {
	return string(b.B)
}

// Reset makes ByteBuffer.B empty.
func (b *ByteBuffer) Reset() {
	b.B = b.B[:0]
	b.Used++
	b.LastAccess = time.Now()
	b.cap = -1
}

func round2(n int) int {
	if n <= 0 {
		return 0
	}

	x := uint32(n - 1)
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16

	// Make sure we don't return 0 due to overflow, even on 32 bit systems
	if x >= uint32(math.MaxInt32) {
		return math.MaxInt32
	}

	return int(x + 1)
}

func (b *ByteBuffer) GrowTo(n int) {
	dst := b.B
	offset := len(dst)
	left := cap(dst) - offset
	dstLen := offset + n
	if left < n {
		b1 := make([]byte, dstLen)
		copy(b1, dst)
		dst = b1
	}
	b.B = dst[:offset]
	b.cap = -1
}
