package mysql

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	User            string
	Password        string
	Host            string
	Port            int
	Database        string
	Charset         string
	ParseTime       bool
	Loc             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        logger.LogLevel
}

var db *gorm.DB

var ErrNotInitialized = errors.New("mysql is not initialized")

func Init(cfg Config) error {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if cfg.Charset == "" {
		cfg.Charset = "utf8mb4"
	}
	if cfg.Loc == "" {
		cfg.Loc = "Local"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 50
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = time.Hour
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 10 * time.Minute
	}
	if cfg.LogLevel == 0 {
		cfg.LogLevel = logger.Warn
	}

	gormDB, err := gorm.Open(mysql.Open(buildDSN(cfg)), &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
	})
	if err != nil {
		return fmt.Errorf("open mysql failed: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get mysql sql db failed: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("ping mysql failed: %w", err)
	}

	if db != nil {
		oldDB, err := db.DB()
		if err == nil {
			_ = oldDB.Close()
		}
	}

	db = gormDB
	return nil
}

func buildDSN(cfg Config) string {
	parseTime := "False"
	if cfg.ParseTime {
		parseTime = "True"
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%s&loc=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
		parseTime,
		cfg.Loc,
	)
}

func DB() (*gorm.DB, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db, nil
}

func MustDB() *gorm.DB {
	gormDB, err := DB()
	if err != nil {
		panic(err)
	}
	return gormDB
}

func Close() error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	err = sqlDB.Close()
	db = nil
	return err
}

func AutoMigrate(models ...any) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.AutoMigrate(models...)
}

func Create[T any](value *T) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.Create(value).Error
}

func Save[T any](value *T) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.Save(value).Error
}

func First[T any](dest *T, query any, args ...any) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.First(dest, query, args...).Error
}

func Find[T any](dest *[]T, query any, args ...any) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}

	if query == nil {
		return gormDB.Find(dest).Error
	}
	return gormDB.Find(dest, query, args...).Error
}

func Delete[T any](value *T, query any, args ...any) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.Delete(value, query, args...).Error
}

func WithTx(fn func(tx *gorm.DB) error) error {
	gormDB, err := DB()
	if err != nil {
		return err
	}
	return gormDB.Transaction(fn)
}
