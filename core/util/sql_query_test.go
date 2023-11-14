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
		name string
		sqlStr  string
		want []string
	}{
		{"simple", "SELECT * FROM tt", []string{"tt"}},
		{"with fields", "SELECT id, name FROM tt", []string{"tt"}},
		{"with quote", `SELECT id, name FROM ".tt"`, []string{".tt"}},
		{"with comma", `SELECT id, name FROM ".tt";`, []string{".tt"}},
		{"with order", `SELECT id, name FROM ".tt" where id='xxx' order by timestamp limit 1;`, []string{".tt"}},
		{"with join", "SELECT users.name, orders.order_date FROM users INNER JOIN orders ON users.id = orders.user_id", []string{"users", "orders"}},
		{"with sub query", `SELECT id, name FROM (select * FROM ".tt" where id='xxx');`, []string{".tt"}},
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
