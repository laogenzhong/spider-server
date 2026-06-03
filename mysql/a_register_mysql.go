package mysql

import (
	"log"
	appconfig "spider-server/common/config"
	"spider-server/mysql/config"
	mysqlmodel2 "spider-server/mysql/model"
	"strings"

	"gorm.io/gorm/logger"
)

func Init() {
	cfg, err := appconfig.LoadDefault()
	if err != nil {
		log.Fatal(err)
		return
	}
	InitWithConfig(cfg.MySQL)
}

func InitWithConfig(mysqlCfg appconfig.MySQLConfig) {
	cfg := config.Config{
		User:            mysqlCfg.User,
		Password:        mysqlCfg.Password,
		Host:            mysqlCfg.Host,
		Port:            mysqlCfg.Port,
		Database:        mysqlCfg.Database,
		Charset:         mysqlCfg.Charset,
		ParseTime:       mysqlCfg.ParseTime,
		Loc:             mysqlCfg.Loc,
		MaxOpenConns:    mysqlCfg.MaxOpenConns,
		MaxIdleConns:    mysqlCfg.MaxIdleConns,
		ConnMaxLifetime: mysqlCfg.ConnMaxLifetimeDuration(),
		ConnMaxIdleTime: mysqlCfg.ConnMaxIdleTimeDuration(),
		LogLevel:        mysqlLogLevel(mysqlCfg.LogLevel),
	}

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

	if err := config.InitAndAutoMigrate(cfg, models...); err != nil {
		log.Fatal(err)
		return
	}
}

func mysqlLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent":
		return logger.Silent
	case "info":
		return logger.Info
	case "warn":
		return logger.Warn
	case "error":
		return logger.Error
	default:
		return logger.Warn
	}
}
