package sqlite

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"infini.sh/framework/core/util"
	"testing"
	"time"
)

type UserInfo struct {
	Uid        int    `gorm:"AUTO_INCREMENT"`
	Count      int    `gorm:"-"`
	Username   string `gorm:"size:255"`
	DepartName string `gorm:"size:255"`
	Created    time.Time
}

type UserGroup struct {
	Count      int
	DepartName string
	Created    time.Time
	Updated    time.Time
}

func TestSmokeTest1(t *testing.T) {
	util.FileDelete("/tmp/test_database12.db")

	fileName := fmt.Sprintf("file:%s?cache=shared&mode=rwc", "/tmp/test_database12.db")
	fmt.Println(fileName)

	db, err := gorm.Open("sqlite3", fileName)
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()
	// Migrate the schema
	db.AutoMigrate(&UserInfo{})

	u := UserInfo{Username: "medcl", DepartName: "dev"}
	db.Create(&u)

	u = UserInfo{Username: "shay", DepartName: "dev"}
	db.Create(&u)

	u = UserInfo{Username: "joe", DepartName: "design"}
	db.Create(&u)

	rows, _ := db.Table("user_infos").Select("depart_name,count(*) as count").Group("depart_name").
		Having("username=?", "medcl").
		Rows()
	columns, _ := rows.Columns()
	fmt.Println(columns)

	g := UserGroup{}

	for rows.Next() {
		db.ScanRows(rows, &g)
		fmt.Println(g)
	}

	db.AutoMigrate(UserGroup{})
	host := UserGroup{}
	host.DepartName = "baidu.com"
	time := time.Now().UTC()
	host.Created = time
	host.Updated = time

	db.Create(&host)
	host = UserGroup{}
	host.DepartName = "baidu.com"
	db.Find(&host)
	fmt.Println(util.ToJson(host, true))

	var us []UserInfo
	db.Model(&u).Where("depart_name=?", "dev").Find(&us)
	fmt.Println(us)

}
