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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetTableNames(t *testing.T) {

	testCases := []struct {
		name   string
		sqlStr string
		want   []string
	}{
		{"simple", "SELECT * FROM tt", []string{"tt"}},
		{"with fields", "SELECT id, name FROM tt", []string{"tt"}},
		{"with quote", `SELECT id, name FROM ".tt"`, []string{`".tt"`}},
		{"with comma", `SELECT id, name FROM ".tt";`, []string{`".tt"`}},
		{"with order", `SELECT id, name FROM ".tt" where id='xxx' order by timestamp limit 1;`, []string{`".tt"`}},
		{"with join", "SELECT users.name, orders.order_date FROM users INNER JOIN orders ON users.id = orders.user_id", []string{"users", "orders"}},
		{"with sub query", `SELECT id, name FROM (select * FROM ".tt" where id='xxx');`, []string{`".tt"`}},
		{"with newline", "\n  SELECT * FROM test56 where x100=100 ORDER BY now_with_format DESC\n", []string{"test56"}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sqlQuery := NewSQLQueryString(tc.sqlStr)
			tableNames, err := sqlQuery.TableNames()
			if err != nil {
				t.Fatalf("could not get table names: %v", err)
			}
			assert.Equal(t, tc.want, tableNames)
		})
	}
}
