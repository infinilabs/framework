/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package stats

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestGoroutinesInfo(t *testing.T) {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)

	stacks := strings.Split(string(buf[:n]), "\n\n")
	grouped := make(map[string]int)
	patternMem,err:=regexp.Compile("\\+?0x[\\d\\w]+")
	if err!=nil{
		panic(err)
	}
	patternID,err:=regexp.Compile("^goroutine \\d+")
	if err!=nil{
		panic(err)
	}

	patternNewID,err:=regexp.Compile("^goroutine ID")
	if err!=nil{
		panic(err)
	}
	for _, stack := range stacks {
		newStack:=patternMem.ReplaceAll([]byte(stack),[]byte("_address_"))
		newStack=patternID.ReplaceAll([]byte(newStack),[]byte("goroutine ID"))
		grouped[string(newStack)]++
	}

	for funcPath, count := range grouped {
		str:=patternNewID.ReplaceAllString(funcPath,fmt.Sprintf("%v same instance of goroutines",count))
		fmt.Printf("%v\n", str)
	}
}
