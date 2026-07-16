package mysqlmodel

import (
	"fmt"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DailyUserActivitySnapshot struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	ActivityDate   time.Time `gorm:"type:date;not null;uniqueIndex:idx_daily_activity_uid"`
	UID            uint64    `gorm:"not null;uniqueIndex:idx_daily_activity_uid;index"`
	LastAppEnterAt time.Time `gorm:"not null;index"`
	CapturedAt     time.Time `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func SnapshotDailyUserActivity(day time.Time, capturedAt time.Time) (int64, error) {
	if day.IsZero() {
		return 0, fmt.Errorf("activity snapshot day is empty")
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now()
	}
	localDay := day.In(time.Local)
	start := time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	db, err := config.DB()
	if err != nil {
		return 0, err
	}

	rows := make([]struct {
		UID            uint64
		LastAppEnterAt time.Time
	}, 0)
	if err := db.Model(&User{}).
		Select("id AS uid, last_app_enter_at").
		Where("deleted_at IS NULL AND last_app_enter_at >= ? AND last_app_enter_at < ?", start, end).
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	records := make([]DailyUserActivitySnapshot, 0, len(rows))
	for _, row := range rows {
		records = append(records, DailyUserActivitySnapshot{
			ActivityDate:   start,
			UID:            row.UID,
			LastAppEnterAt: row.LastAppEnterAt,
			CapturedAt:     capturedAt,
		})
	}
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "activity_date"}, {Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_app_enter_at": gorm.Expr("VALUES(last_app_enter_at)"),
			"captured_at":       capturedAt,
			"updated_at":        time.Now(),
		}),
	}).CreateInBatches(records, 500)
	return result.RowsAffected, result.Error
}
