package mysqlmodel

import (
	"fmt"
	"spider-server/mysql/config"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserSession struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	UID       uint64 `gorm:"not null;uniqueIndex:idx_uid_scope"`
	ScopeID   uint64 `gorm:"not null;uniqueIndex:idx_uid_scope"`
	Attach    string `gorm:"type:text;not null"`
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func InitUserSessionTable() error {
	return config.AutoMigrate(&UserSession{})
}

func CreateOrUpdateUserSession(uid uint64, scopeID uint64, attach string, expiresAt *time.Time) (*UserSession, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	session := &UserSession{
		UID:       uid,
		ScopeID:   scopeID,
		Attach:    attach,
		ExpiresAt: expiresAt,
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	err = db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "uid"},
			{Name: "scope_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"attach":     attach,
			"expires_at": expiresAt,
			"deleted_at": nil,
			"updated_at": time.Now(),
		}),
	}).Create(session).Error

	if err != nil {
		return nil, err
	}

	return GetUserSession(uid, scopeID)
}

func CreateUserSession(uid uint64, scopeID uint64, attach string, expiresAt *time.Time) (*UserSession, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	session := &UserSession{
		UID:       uid,
		ScopeID:   scopeID,
		Attach:    attach,
		ExpiresAt: expiresAt,
	}

	if err := config.Create(session); err != nil {
		return nil, err
	}

	return session, nil
}

func GetUserSession(uid uint64, scopeID uint64) (*UserSession, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	session := &UserSession{}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Where("uid = ? AND scope_id = ?", uid, scopeID).First(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

func UpdateUserSession(uid uint64, scopeID uint64, attach string, expiresAt *time.Time) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.Model(&UserSession{}).
		Where("uid = ? AND scope_id = ?", uid, scopeID).
		Updates(map[string]any{
			"attach":     attach,
			"expires_at": expiresAt,
		}).Error
}

func DeleteUserSession(uid uint64, scopeID uint64) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}

	return config.Delete(&UserSession{}, "uid = ? AND scope_id = ?", uid, scopeID)
}

func DeleteExpiredUserSessions() error {
	db, err := config.DB()
	if err != nil {
		return err
	}

	return db.
		Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).
		Delete(&UserSession{}).
		Error
}
