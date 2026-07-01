package mysqlmodel

import (
	"fmt"
	"spider-server/mysql/config"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DefaultWeeklyStrengthGoal int32 = 2
	DefaultWeeklyCardioGoal   int32 = 1
	MaxWeeklyTrainingGoal     int32 = 14
)

// WeeklyTrainingGoal 表示用户每周训练目标。
//
// UID 唯一，保证每个用户只有一份目标配置。
type WeeklyTrainingGoal struct {
	ID               uint64         `gorm:"primaryKey;autoIncrement"`
	UID              uint64         `gorm:"not null;uniqueIndex"`
	StrengthSessions int32          `gorm:"not null"`
	CardioSessions   int32          `gorm:"not null"`
	CreatedAt        time.Time      `gorm:"not null"`
	UpdatedAt        time.Time      `gorm:"not null"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func DefaultWeeklyTrainingGoal(uid uint64) *WeeklyTrainingGoal {
	return &WeeklyTrainingGoal{
		UID:              uid,
		StrengthSessions: DefaultWeeklyStrengthGoal,
		CardioSessions:   DefaultWeeklyCardioGoal,
	}
}

func GetWeeklyTrainingGoal(uid uint64) (*WeeklyTrainingGoal, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	record := &WeeklyTrainingGoal{}
	if err := db.Where("uid = ?", uid).First(record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return DefaultWeeklyTrainingGoal(uid), nil
		}
		return nil, err
	}
	if !HasValidWeeklyTrainingGoalTotal(record.StrengthSessions, record.CardioSessions) {
		return DefaultWeeklyTrainingGoal(uid), nil
	}
	return record, nil
}

func HasValidWeeklyTrainingGoalTotal(strengthSessions int32, cardioSessions int32) bool {
	return strengthSessions+cardioSessions > 0
}

func SaveWeeklyTrainingGoal(uid uint64, strengthSessions int32, cardioSessions int32) (*WeeklyTrainingGoal, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	record := &WeeklyTrainingGoal{
		UID:              uid,
		StrengthSessions: strengthSessions,
		CardioSessions:   cardioSessions,
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"strength_sessions": record.StrengthSessions,
			"cardio_sessions":   record.CardioSessions,
			"deleted_at":        nil,
			"updated_at":        time.Now(),
		}),
	}).Create(record).Error; err != nil {
		return nil, err
	}

	return GetWeeklyTrainingGoal(uid)
}
