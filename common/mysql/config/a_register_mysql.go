package mysqlconfig

import (
	"spider-server/common/mysql"
	mysqlmodel "spider-server/common/mysql/model"
)

func Init() {
	cfg := mysql.Config{User: "root", Password: "root", Host: "localhost", Port: 3306, Database: "spider"}
	err := mysql.InitAndAutoMigrate(cfg, &mysqlmodel.User{})
	if err != nil {
		return
	}
}
