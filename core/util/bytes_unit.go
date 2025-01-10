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

package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	B  float64 = 1
	KB float64 = 1024
	MB         = 1024 * KB
	GB         = 1024 * MB
	TB         = 1024 * GB
	PB         = 1024 * TB
)

func ConvertBytesFromString(formatedBytes string) (float64, error) {
	runes := []rune(formatedBytes)
	splitIdx := 0
	for i, ch := range runes {
		if ch > 57 {
			splitIdx = i
			break
		}
	}
	bytesUnit := strings.ToLower(string(runes[splitIdx:]))
	bytesValue := string(runes[0:splitIdx])
	if bytesValue == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(bytesValue, 64)
	if err != nil {
		return 0, err
	}
	unitValues := map[string]float64{
		"b":  B,
		"kb": KB,
		"mb": MB,
		"gb": GB,
		"tb": TB,
	}
	if uv, ok := unitValues[bytesUnit]; ok {
		return value * uv, nil
	}
	return value, nil
}

func FormatBytes(bytes float64, precision int) string {
	units := []string{"b", "kb", "mb", "gb", "tb", "pb"}
	if bytes <= 0 {
		return "0b"
	}
	var idx int

	for {
		if bytes < 1024 || idx >= len(units) {
			break
		}
		bytes = bytes / 1024
		idx++
	}

	d := float64(1)
	if precision > 0 {
		d = math.Pow10(precision)
	}
	bytesStr := strconv.FormatFloat(math.Trunc(bytes*d)/d, 'f', -1, 64)
	return fmt.Sprintf("%s%s", bytesStr, units[idx])
}
