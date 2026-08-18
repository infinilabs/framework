/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"testing"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/orm/ormtest"
)

// TestContract_SQLite runs the shared backend contract suite; elastic wires
// the same suite behind the `integration` build tag where a live cluster is
// available.
func TestContract_SQLite(t *testing.T) {
	ormtest.RunContractTests(t, func() orm.ORM {
		handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: t.TempDir() + "/contract.db"}}
		if err := handler.Open(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { handler.Close() })
		return handler
	})
}
