package mysql

import (
	"log"
	"spider-server/mysql/config"
	mysqlmodel2 "spider-server/mysql/model"
)

func Init() {
	cfg := config.Config{User: "root", Password: "root", Host: "localhost", Port: 3306, Database: "spider", ParseTime: true}

	models := []any{
		&mysqlmodel2.User{},
		&mysqlmodel2.UserSession{},
		&mysqlmodel2.WeightRecord{},
		&mysqlmodel2.TrainingTag{},
		&mysqlmodel2.WorkoutTagBinding{},
		&mysqlmodel2.BodyPhotoRecord{},
		&mysqlmodel2.FriendProfileRecord{},
		&mysqlmodel2.FriendRequestRecord{},
		&mysqlmodel2.FriendRelationRecord{},
		&mysqlmodel2.FriendRemarkRecord{},
	}

	for _, model := range models {
		if err := config.InitAndAutoMigrate(cfg, model); err != nil {
			log.Fatal(err)
			return
		}
	}
}
