package elastic

import (
	"fmt"
	"testing"
)

func TestGetHash(t *testing.T) {
	id:=GetShardID(7,[]byte("20210811_1_61de2c1369300ff4089b70422c65ad24"),8)
	fmt.Println(id)
}