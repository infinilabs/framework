/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package util

import (
	"fmt"
	"testing"
	"time"
)

func TestUnixTime(t *testing.T) {
	t1 := time.Now().Unix()
	t2 := time.Now().UnixNano()
	fmt.Println(t1)
	fmt.Println(t2)
}

func TestGetLowPrecisionCurrentTime(t *testing.T) {
	SetupTimeNowRefresh()
	for i := 0; i < 10; i++ {
		t1 := GetLowPrecisionCurrentTime()
		fmt.Println(t1.String())
		time.Sleep(500 * time.Millisecond)
	}

}
