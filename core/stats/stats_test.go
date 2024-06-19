/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package stats

import "testing"

func TestJoinArray(t *testing.T) {
	array := []string{"a", "b", "c", "d", "e"}
	separator := ","
	expected := "a,b,c,d,e"
	actual := JoinArray(array, separator)
	if actual != expected {
		t.Errorf("Expected %s but got %s", expected, actual)
	}

	array = []string{"a"}
	expected = "a"
	actual = JoinArray(array, separator)
	if actual != expected {
		t.Errorf("Expected %s but got %s", expected, actual)
	}

}