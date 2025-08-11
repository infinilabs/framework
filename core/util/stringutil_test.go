// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Helper function to compare slices for testing StringArrayIntersection
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]int)
	bMap := make(map[string]int)
	for _, v := range a {
		aMap[v]++
	}
	for _, v := range b {
		bMap[v]++
	}
	return reflect.DeepEqual(aMap, bMap)
}

func TestMergeSpacke(t *testing.T) {
	str := "hello world     !"
	str = MergeSpace(str)
	assert.Equal(t, "hello world !", str)

	str = " hello world  \n \r  !"
	str = MergeSpace(str)
	assert.Equal(t, "hello world !", str)
}

func TestTrimSpaces(t *testing.T) {
	str := " left"
	assert.Equal(t, "left", TrimSpaces(str))

	str = "right "
	assert.Equal(t, "right", TrimSpaces(str))

	str = " side "
	assert.Equal(t, "side", TrimSpaces(str))

	str = "midd le"
	assert.Equal(t, "middle", RemoveSpaces(str))
}

func TestRemoveSpaces(t *testing.T) {
	str := " left"
	assert.Equal(t, "left", RemoveSpaces(str))

	str = "right "
	assert.Equal(t, "right", RemoveSpaces(str))

	str = " side "
	assert.Equal(t, "side", RemoveSpaces(str))

	str = "midd le"
	assert.Equal(t, "middle", RemoveSpaces(str))
}

func TestTrimLeft(t *testing.T) {
	str := "static://off"
	new := TrimLeftStr(str, "static:")
	assert.Equal(t, "//off", new)

	str = "/static/static://off"
	new = TrimLeftStr(str, "/static")
	assert.Equal(t, "/static://off", new)
}

func TestToInt64(t *testing.T) {
	str := "6393600409"
	new, _ := ToInt64(str)
	fmt.Println(new)
}

func TestContainsAnyInAnyIntArray(t *testing.T) {
	str := "key"
	strs := []interface{}{}
	strs = append(strs, "key", "key1")
	ok := ContainsAnyInAnyIntArray(str, strs)
	fmt.Println(ok)
	assert.Equal(t, true, ok)

	str = "key3"
	ok = ContainsAnyInAnyIntArray(str, strs)
	fmt.Println(ok)
	assert.Equal(t, false, ok)

	str1 := 5
	strs2 := []interface{}{}
	strs2 = append(strs2, 5, 6)
	ok2 := ContainsAnyInAnyIntArray(str1, strs2)
	fmt.Println(ok2)
	assert.Equal(t, true, ok2)

	str1 = 7
	ok2 = ContainsAnyInAnyIntArray(str1, strs2)
	fmt.Println(ok2)
	assert.Equal(t, false, ok2)
}

func TestContainsString(t *testing.T) {
	a := "{\"error\":true,\"message\":\"timeout\"}"
	b := "\"message\":\"timeout\""
	ok := ContainStr(a, b)
	fmt.Println(ok)
	assert.Equal(t, true, ok)
}

func TestInt64ToString(t *testing.T) {
	i := 6393600409
	new := Int64ToString(int64(i))
	fmt.Println(new)
	assert.Equal(t, "6393600409", new)

	i = 63
	new = Int64ToString(int64(i))
	fmt.Println(new)
	assert.Equal(t, "63", new)
}

func TestStringDefault(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		defaultV string
		want     string
	}{
		{"WithValue", "hello", "world", "hello"},
		{"WithoutValue", "", "world", "world"},
		{"BothEmpty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringDefault(tt.val, tt.defaultV); got != tt.want {
				t.Errorf("StringDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterfaceToInt(t *testing.T) {
	tests := []struct {
		name string
		y    interface{}
		want int
	}{
		{"int", 10, 10},
		{"int8", int8(10), 10},
		{"int16", int16(10), 10},
		{"int32", int32(10), 10},
		{"int64", int64(10), 10},
		{"uint", uint(10), 10},
		{"uint8", uint8(10), 10},
		{"uint16", uint16(10), 10},
		{"uint32", uint32(10), 10},
		{"uint64", uint64(10), 10},
		{"float32", float32(10.9), 10},
		{"float64", float64(10.9), 10},
		{"string", "not a number", 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InterfaceToInt(tt.y); got != tt.want {
				t.Errorf("InterfaceToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareInteger(t *testing.T) {
	tests := []struct {
		name string
		x    interface{}
		y    interface{}
		want bool
	}{
		{"int vs int64", 5, int64(5), true},
		{"float vs int", 5.0, 5, true},
		{"different numbers", 5, 6, false},
		{"one nil", 5, nil, false},
		{"two nil", nil, nil, false}, // The function returns false if any is nil
		{"identical strings", "hello", "hello", true},
		{"different strings", "hello", "world", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareInteger(tt.x, tt.y); got != tt.want {
				t.Errorf("CompareInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsAnyInArray(t *testing.T) {
	tests := []struct {
		name string
		s    string
		v    []string
		want bool
	}{
		{"Contains", "hello world", []string{"earth", "world"}, true},
		{"NotContains", "hello world", []string{"earth", "mars"}, false},
		{"EmptySlice", "hello world", []string{}, false},
		{"EmptyString", "", []string{"a", "b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsAnyInArray(tt.s, tt.v); got != tt.want {
				t.Errorf("ContainsAnyInArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubStringWithSuffix(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		length int
		suffix string
		want   string
	}{
		{"String is longer", "hello world", 5, "...", "hello..."},
		{"String is shorter", "hi", 5, "...", "hi"},
		{"String is equal", "hello", 5, "...", "hello"},
		{"Zero length", "hello world", 0, "...", "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubStringWithSuffix(tt.str, tt.length, tt.suffix); got != tt.want {
				t.Errorf("SubStringWithSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnicodeIndex(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		substr string
		want   int
	}{
		{"ASCII", "hello world", "world", 6},
		{"Unicode", "你好，世界", "世界", 3},
		{"NotFound", "你好，世界", "地球", -1},
		{"Empty Substr", "你好", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UnicodeIndex(tt.str, tt.substr); got != tt.want {
				t.Errorf("UnicodeIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeSpace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"Multiple spaces", "hello   world", "hello world"},
		{"Leading and trailing", "  hello world  ", "hello world"},
		{"Tabs and newlines", "hello\tworld\nnow", "hello world now"},
		{"No spaces", "helloworld", "helloworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeSpace(tt.in); got != tt.want {
				t.Errorf("MergeSpace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToJson(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	s := sample{Name: "tester", Age: 99}

	tests := []struct {
		name   string
		in     interface{}
		indent bool
		want   string
	}{
		{"NoIndent", s, false, `{"name":"tester","age":99}`},
		{"WithIndent", s, true, "{\n \"name\": \"tester\",\n \"age\": 99\n}"}, // Note: JSON indentation can vary slightly
		{"NilInput", nil, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToJson(tt.in, tt.indent)
			// Simple replace to make test stable against minor formatting changes
			got = strings.ReplaceAll(got, " ", "")
			want := strings.ReplaceAll(tt.want, " ", "")
			if got != want {
				t.Errorf("ToJson() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromJson(t *testing.T) {
	type sample struct {
		Name string
		Age  int
	}
	t.Run("ValidJSON", func(t *testing.T) {
		var s sample
		err := FromJson(`{"Name":"tester","Age":99}`, &s)
		if err != nil {
			t.Errorf("FromJson() unexpected error: %v", err)
		}
		if s.Name != "tester" || s.Age != 99 {
			t.Errorf("FromJson() got = %v, want %v", s, sample{Name: "tester", Age: 99})
		}
	})
	t.Run("InvalidJSON", func(t *testing.T) {
		var s sample
		err := FromJson(`{"Name":"tester"`, &s)
		if err == nil {
			t.Error("FromJson() expected an error, but got nil")
		}
	})
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name    string
		str     string
		want    int
		wantErr bool
	}{
		{"Valid integer", "123", 123, false},
		{"Valid with float part", "123.45", 123, false},
		{"Negative", "-50", -50, false},
		{"Invalid", "abc", 0, true},
		{"Empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToInt(tt.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSpecialChar(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		want    string
	}{
		{"Mixed Chars", `hel"lo+world/test:[]{}`, "hel lo world test"},
		{"Multiple Spaces", `a 	 b`, "a b"},
		{"No Special Chars", "helloworld", "helloworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterSpecialChar(tt.keyword); got != tt.want {
				t.Errorf("FilterSpecialChar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMD5digest(t *testing.T) {
	t.Run("KnownValue", func(t *testing.T) {
		// MD5 of "hello" is 5d41402abc4b2a76b9719d911017c592
		want := "5d41402abc4b2a76b9719d911017c592"
		if got := MD5digest("hello"); got != want {
			t.Errorf("MD5digest() = %v, want %v", got, want)
		}
	})
}

func TestJoinArray(t *testing.T) {
	tests := []struct {
		name      string
		array     []string
		delimiter string
		want      string
	}{
		{"Simple join", []string{"a", "b", "c"}, ",", "a,b,c"},
		{"Empty array", []string{}, ",", ""},
		{"Single element", []string{"a"}, ",", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinArray(tt.array, tt.delimiter); got != tt.want {
				t.Errorf("JoinArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name    string
		v1      string
		v2      string
		want    int
		wantErr bool
	}{
		{"v1 < v2", "1.2.3", "1.2.4", -1, false},
		{"v1 == v2", "1.2.3", "1.2.3", 0, false},
		{"v1 > v2", "2.0.0", "1.9.9", 1, false},
		{"Invalid v1", "invalid", "1.0.0", -2, true},
		{"Invalid v2", "1.0.0", "invalid", -2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VersionCompare(tt.v1, tt.v2)
			if (err != nil) != tt.wantErr {
				t.Errorf("VersionCompare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VersionCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name    string
		cnum    int
		wantLen int
	}{
		{"Positive length", 10, 10},
		{"Zero length", 0, 0},
		{"Negative length", -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRandomString(tt.cnum)
			if len(got) != tt.wantLen {
				t.Errorf("GenerateRandomString() length = %v, want %v", len(got), tt.wantLen)
			}
			// Check if only contains allowed characters
			if len(got) > 0 {
				if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(got) {
					t.Errorf("GenerateRandomString() got = %v, contains invalid characters", got)
				}
			}
		})
	}
}

func TestGenerateSecureString(t *testing.T) {
	tests := []struct {
		name    string
		cnum    int
		wantLen int
	}{
		{"Sufficient length", 12, 12},
		{"Insufficient length", 5, 8}, // Should default to 8
		{"Minimum length", 8, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSecureString(tt.cnum)
			if len(got) != tt.wantLen {
				t.Errorf("GenerateSecureString() length = %v, want %v", len(got), tt.wantLen)
				return
			}

			// Validate content for complexity
			hasLower := regexp.MustCompile(`[a-z]`).MatchString(got)
			hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(got)
			hasDigit := regexp.MustCompile(`[0-9]`).MatchString(got)
			// Escape special characters for regex
			specialRegex := regexp.QuoteMeta("!@#$%^&*()-_=+[]{}|;:',.<>?/")
			hasSpecial := regexp.MustCompile(`[` + specialRegex + `]`).MatchString(got)

			if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
				t.Errorf("GenerateSecureString() = %v, failed complexity requirements (lower: %t, upper: %t, digit: %t, special: %t)", got, hasLower, hasUpper, hasDigit, hasSpecial)
			}
		})
	}
}

func TestStringArrayIntersection(t *testing.T) {
	tests := []struct {
		name string
		arr1 []string
		arr2 []string
		want []string
	}{
		{"Has intersection", []string{"a", "b", "c"}, []string{"b", "d", "a"}, []string{"a", "b"}},
		{"No intersection", []string{"a", "b"}, []string{"c", "d"}, []string{}},
		{"One empty", []string{"a", "b"}, []string{}, []string{}},
		{"Both empty", []string{}, []string{}, []string{}},
		{"Identical", []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringArrayIntersection(tt.arr1, tt.arr2); !slicesEqual(got, tt.want) {
				t.Errorf("StringArrayIntersection() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateSecure validates the unified validation function in all its modes.
func TestValidateSecure(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		secureArgs []bool // Used to simulate the variadic parameter
		want       bool
	}{
		// --- Group 1: Testing Secure Mode (Default or secure=true) ---
		{name: "Secure Valid (Default)", input: "Password123!", secureArgs: nil, want: true},
		{name: "Secure Valid (Explicit)", input: "Password123!", secureArgs: []bool{true}, want: true},
		{name: "Secure Too Short", input: "Pass1!", secureArgs: nil, want: false},
		{name: "Secure Missing Upper", input: "password123!", secureArgs: nil, want: false},
		{name: "Secure Missing Lower", input: "PASSWORD123!", secureArgs: nil, want: false},
		{name: "Secure Missing Digit", input: "Password!", secureArgs: nil, want: false},
		{name: "Secure Missing Special", input: "Password123", secureArgs: nil, want: false},
		{name: "Secure Empty", input: "", secureArgs: nil, want: false},

		// --- Group 2: Testing Simple Mode (secure=false) ---
		{name: "Simple Valid (lower and digit)", input: "password123", secureArgs: []bool{false}, want: true},
		{name: "Simple Valid (with special chars)", input: "password123!", secureArgs: []bool{false}, want: true},
		{name: "Simple Valid (with upper chars)", input: "Password123", secureArgs: []bool{false}, want: true},
		{name: "Simple Missing Digit", input: "input", secureArgs: []bool{false}, want: false},
		{name: "Simple Missing Lower", input: "12345678", secureArgs: []bool{false}, want: false},
		{name: "Simple Empty", input: "", secureArgs: []bool{false}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The '...' unpacks the slice into the variadic function call.
			got := ValidateSecure(tt.input, tt.secureArgs...)
			if got != tt.want {
				t.Errorf("ValidateSecure(\"%s\", ...) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
