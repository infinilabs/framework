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

//func TestGetLowPrecisionCurrentTime(t *testing.T) {
//	SetupTimeNowRefresh()
//	for i := 0; i < 10; i++ {
//		t1 := GetLowPrecisionCurrentTime()
//		fmt.Println(t1.String())
//		time.Sleep(500 * time.Millisecond)
//	}
//
//}

func TestFromUnixTimestamp(t *testing.T) {
	str := GetLowPrecisionCurrentTime().Unix()

	fmt.Println(str)

	timestamp := FromUnixTimestamp(str)

	fmt.Println(timestamp)
}

func TestParseDuration(t *testing.T) {
	var tests = []struct {
		str  string
		want int64
	}{
		{"10ms", int64(time.Millisecond) * 10},
		{"10s", int64(time.Second) * 10},
		{"10m", int64(time.Minute) * 10},
		{"10h", int64(time.Hour) * 10},
		{"10d", int64(time.Hour) * 24 * 10},
		{"2w", int64(time.Hour) * 24 * 14},
		{"2M", int64(time.Hour) * 24 * 30 * 2},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			ans, err := ParseDuration(tt.str)
			if err != nil {
				t.Errorf("got error: %v", err)
			}
			if int64(ans) != tt.want {
				t.Errorf("got %d, want %d", ans, tt.want)
			}
		})
	}
}
