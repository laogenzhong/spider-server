package mysqlmodel

import (
	"fmt"
	pb "spider-server/gen/spider/api"
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

func CountWeeklyTrainingGoalChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, error) {
	if uid == 0 {
		return 0, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return 0, fmt.Errorf("snapshot range is invalid")
	}

	db, err := config.DB()
	if err != nil {
		return 0, err
	}

	var count int64
	if err := weeklyTrainingGoalChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&WeeklyTrainingGoal{}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return uint64(count), nil
}

func ListWeeklyTrainingGoalChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*WeeklyTrainingGoal, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if endSnapshotID <= 0 || startSnapshotID > endSnapshotID {
		return nil, fmt.Errorf("snapshot range is invalid")
	}
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var records []*WeeklyTrainingGoal
	if err := weeklyTrainingGoalChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order("GREATEST(updated_at, COALESCE(deleted_at, updated_at)) ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func WeeklyTrainingGoalToPB(goal *WeeklyTrainingGoal) *pb.WeeklyTrainingGoal {
	if goal == nil {
		return nil
	}

	return &pb.WeeklyTrainingGoal{
		Id:               goal.ID,
		Uid:              goal.UID,
		StrengthSessions: goal.StrengthSessions,
		CardioSessions:   goal.CardioSessions,
		CreatedAt:        goal.CreatedAt.UnixMilli(),
		UpdatedAt:        goal.UpdatedAt.UnixMilli(),
	}
}

func weeklyTrainingGoalChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
	endTime := time.UnixMilli(endSnapshotID)
	query := db.Unscoped().Where("uid = ?", uid)
	if startSnapshotID <= 0 {
		return query.Where("created_at <= ? AND (deleted_at IS NULL OR deleted_at > ?)", endTime, endTime)
	}

	startTime := time.UnixMilli(startSnapshotID)
	return query.Where(
		"(created_at > ? AND created_at <= ?) OR (updated_at > ? AND updated_at <= ?) OR (deleted_at IS NOT NULL AND deleted_at > ? AND deleted_at <= ?)",
		startTime,
		endTime,
		startTime,
		endTime,
		startTime,
		endTime,
	)
}
