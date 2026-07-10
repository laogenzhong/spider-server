package mysqlmodel

import (
	"errors"
	"fmt"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserFeedback 表示用户在 App 内提交的反馈。
type UserFeedback struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"`
	UID       uint64         `gorm:"not null;index:idx_feedback_uid_created_at"`
	Content   string         `gorm:"type:varchar(1200);not null"`
	CreatedAt time.Time      `gorm:"not null;index:idx_feedback_uid_created_at"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

const (
	MaxFeedbackContentLength = 1000
	MaxFeedbackCreatesPerDay = 5
)

var (
	ErrFeedbackContentEmpty             = errors.New("feedback content empty")
	ErrFeedbackContentTooLong           = errors.New("feedback content too long")
	ErrFeedbackDailyCreateLimitExceeded = errors.New("feedback daily create limit exceeded")
)

// CreateUserFeedback 保存一条用户反馈，并限制同一用户每天最多提交 5 次。
func CreateUserFeedback(uid uint64, content string) (*UserFeedback, int, error) {
	if uid == 0 {
		return nil, 0, fmt.Errorf("uid is empty")
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, 0, ErrFeedbackContentEmpty
	}
	if len([]rune(content)) > MaxFeedbackContentLength {
		return nil, 0, ErrFeedbackContentTooLong
	}

	db, err := config.DB()
	if err != nil {
		return nil, 0, err
	}

	record := &UserFeedback{
		UID:     uid,
		Content: content,
	}
	usedToday := 0

	err = db.Transaction(func(tx *gorm.DB) error {
		startAt, endAt := dayBoundsTime(time.Now())
		var dailyRecords []*UserFeedback
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uid = ? AND created_at >= ? AND created_at < ?", uid, startAt, endAt).
			Find(&dailyRecords).Error; err != nil {
			return err
		}

		usedToday = len(dailyRecords)
		if usedToday >= MaxFeedbackCreatesPerDay {
			return ErrFeedbackDailyCreateLimitExceeded
		}

		if err := tx.Create(record).Error; err != nil {
			return err
		}
		usedToday++
		return nil
	})
	if err != nil {
		return nil, usedToday, err
	}

	return record, usedToday, nil
}
