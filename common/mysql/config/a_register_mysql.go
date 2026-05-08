package config

import (
	"spider-server/common/mysql"
	"spider-server/common/mysql/model"
)

func Init(cfg mysql.Config) {
	err := mysql.InitAndAutoMigrate(cfg, &model.User{})
	if err != nil {
		return
	}
}
