package mysqlmodel

import (
	"encoding/json"
	"fmt"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OnboardingProfile stores the health and training context collected during onboarding.
//
// ProfileJSON keeps the original client payload so new questionnaire fields can be added
// without a server schema migration for every onboarding copy change.
type OnboardingProfile struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement"`
	UID           uint64         `gorm:"not null;uniqueIndex"`
	SchemaVersion int            `gorm:"not null;default:1"`
	CompletedAt   time.Time      `gorm:"not null;index"`
	ProfileJSON   string         `gorm:"type:json;not null"`
	CreatedAt     time.Time      `gorm:"not null"`
	UpdatedAt     time.Time      `gorm:"not null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func SaveOnboardingProfile(uid uint64, profileJSON []byte, schemaVersion int, completedAt time.Time) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if len(profileJSON) == 0 {
		return fmt.Errorf("profile json is empty")
	}
	if !json.Valid(profileJSON) {
		return fmt.Errorf("profile json is invalid")
	}
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	if completedAt.IsZero() {
		completedAt = time.Now()
	}

	record := &OnboardingProfile{
		UID:           uid,
		SchemaVersion: schemaVersion,
		CompletedAt:   completedAt,
		ProfileJSON:   string(profileJSON),
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"schema_version": schemaVersion,
			"completed_at":   completedAt,
			"profile_json":   string(profileJSON),
			"deleted_at":     nil,
			"updated_at":     time.Now(),
		}),
	}).Create(record).Error
}
