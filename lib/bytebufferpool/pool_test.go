package bytebufferpool

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPoolVariousSizesSerial(t *testing.T) {
	testPoolVariousSizes(t)
}

func TestPoolVariousSizesConcurrent(t *testing.T) {
	concurrency := 5
	ch := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		go func() {
			testPoolVariousSizes(t)
			ch <- struct{}{}
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case <-ch:
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testPoolVariousSizes(t *testing.T) {
	for i := 0; i < steps+1; i++ {
		n := (1 << uint32(i))

		testGetPut(t, n)
		testGetPut(t, n+1)
		testGetPut(t, n-1)

		for j := 0; j < 10; j++ {
			testGetPut(t, j+n)
		}
	}
}

func testGetPut(t *testing.T, n int) {
	bb := Get("test")
	if len(bb.B) > 0 {
		t.Fatalf("non-empty byte buffer returned from acquire")
	}
	bb.B = allocNBytes(bb.B, n)
	Put("test", bb)
}

func allocNBytes(dst []byte, n int) []byte {
	diff := n - cap(dst)
	if diff <= 0 {
		return dst[:n]
	}
	return append(dst, make([]byte, diff)...)
}

func TestCalibrate(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in CI environment")
	}

	p := getPoolByTag("test")
	for i := 0; i < 1000; i++ {
		x := p.Get()
		x.GrowTo(i)
	}
	t.Log(p.poolItems)
	p.calibrate()
	assert.Equal(t, p.maxItemSize, uint32(999))
}
