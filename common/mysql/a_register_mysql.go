package mysql

import (
	"log"
	"spider-server/common/mysql/config"
	mysqlmodel "spider-server/common/mysql/model"
)

func Init() {
	cfg := config.Config{User: "root", Password: "root", Host: "localhost", Port: 3306, Database: "spider", ParseTime: true}
	err := config.InitAndAutoMigrate(cfg, &mysqlmodel.User{})
	if err != nil {
		log.Fatal(err)
		return
	}

	err = config.InitAndAutoMigrate(cfg, &mysqlmodel.UserSession{})
	if err != nil {
		log.Fatal(err)
		return
	}
}
