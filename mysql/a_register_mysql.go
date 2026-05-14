package mysql

import (
	"log"
	"spider-server/mysql/config"
	mysqlmodel2 "spider-server/mysql/model"
)

func Init() {
	cfg := config.Config{User: "root", Password: "root", Host: "localhost", Port: 3306, Database: "spider", ParseTime: true}
	err := config.InitAndAutoMigrate(cfg, &mysqlmodel2.User{})
	if err != nil {
		log.Fatal(err)
		return
	}

	err = config.InitAndAutoMigrate(cfg, &mysqlmodel2.UserSession{})
	if err != nil {
		log.Fatal(err)
		return
	}

	err = config.InitAndAutoMigrate(cfg, &mysqlmodel2.WeightRecord{})
	if err != nil {
		log.Fatal(err)
		return
	}
}
