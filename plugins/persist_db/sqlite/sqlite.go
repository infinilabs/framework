package sqlite

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"infini.sh/framework/core/global"
	"os"
	"path"
)

// SQLiteConfig currently do nothing
type SQLiteConfig struct {
}

// GetInstance return sqlite instance for further access
func GetInstance(cfg *SQLiteConfig) *gorm.DB {
	os.MkdirAll(path.Join(global.Env().GetWorkingDir(), "database/"), 0755)
	fileName := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=50000000", path.Join(global.Env().GetWorkingDir(), "database/db.sqlite"))

	var err error
	db, err := gorm.Open("sqlite3", fileName)
	if err != nil {
		panic("failed to connect database")
	}
	return db
}
