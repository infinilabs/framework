package host

import (
	"fmt"
	"testing"
	"time"
)

func TestHardwareInfo(t *testing.T) {
	fmt.Println(GetOSInfo())
	fmt.Println(time.Unix(1660824818, 0))
}
