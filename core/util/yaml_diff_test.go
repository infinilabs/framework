/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package util

import (
	"fmt"
	"testing"
	"time"
)

func TestDiffYaml(t *testing.T) {

	yaml1:=map[string]string{}
	yaml2:=map[string]string{}
	yaml1["age"]="15"
	yaml2["age"]="16"
	log,err:=DiffTwoObject(yaml1,yaml2)
	fmt.Println(log)
	fmt.Println(err)

	yaml3:=map[string]time.Time{}
	yaml4:=map[string]time.Time{}
	t3:=time.Now()
	t4:=time.Now()
	yaml3["age"]=t3
	yaml4["age"]=t4

	log,err=DiffTwoObject(yaml3,yaml4)
	fmt.Println(log)
	fmt.Println(err)
}
