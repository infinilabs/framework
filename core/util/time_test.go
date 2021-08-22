/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package util

import (
	"fmt"
	"testing"
	"time"
)

func TestUnixTime(t *testing.T) {
	t1:=time.Now().Unix()
	t2:=time.Now().UnixNano()
	fmt.Println(t1)
	fmt.Println(t2)
}
