package mysqlmodel

import (
	"fmt"
	"strings"
	"time"

	pb "spider-server/gen/spider/api"
	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DefaultUserPreferencesTheme                 = "liftTags"
	DefaultUserPreferencesThemeOpacity    int32 = 80
	DefaultUserPreferencesCalendarPalette       = "classic"
	DefaultUserPreferencesSchemaVersion   int32 = 1
)

// UserPreferences stores user-level appearance and UI preferences.
type UserPreferences struct {
	ID                        uint64         `gorm:"primaryKey;autoIncrement"`
	UID                       uint64         `gorm:"not null;uniqueIndex"`
	Theme                     string         `gorm:"size:32;not null;default:'liftTags'"`
	ThemeOpacity              int32          `gorm:"not null;default:80"`
	CalendarLightPalette      string         `gorm:"size:32;not null;default:'classic'"`
	CalendarDarkPalette       string         `gorm:"size:32;not null;default:'classic'"`
	HidesNavigationTitleSpark bool           `gorm:"not null;default:false"`
	SchemaVersion             int32          `gorm:"not null;default:1"`
	CreatedAt                 time.Time      `gorm:"not null"`
	UpdatedAt                 time.Time      `gorm:"not null"`
	DeletedAt                 gorm.DeletedAt `gorm:"index"`
}

func SaveUserPreferences(uid uint64, prefs *pb.UserPreferences) (*UserPreferences, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	if prefs == nil {
		return nil, fmt.Errorf("preferences is nil")
	}

	record := normalizeUserPreferences(uid, prefs)

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"theme":                        record.Theme,
			"theme_opacity":                record.ThemeOpacity,
			"calendar_light_palette":       record.CalendarLightPalette,
			"calendar_dark_palette":        record.CalendarDarkPalette,
			"hides_navigation_title_spark": record.HidesNavigationTitleSpark,
			"schema_version":               record.SchemaVersion,
			"deleted_at":                   nil,
			"updated_at":                   now,
		}),
	}).Create(record).Error; err != nil {
		return nil, err
	}

	return GetUserPreferences(uid)
}

func GetUserPreferences(uid uint64) (*UserPreferences, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	record := &UserPreferences{}
	if err := db.Where("uid = ?", uid).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func CountUserPreferencesChanges(uid uint64, startSnapshotID int64, endSnapshotID int64) (uint64, error) {
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
	if err := userPreferencesChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Model(&UserPreferences{}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return uint64(count), nil
}

func ListUserPreferencesChangesPage(uid uint64, startSnapshotID int64, endSnapshotID int64, limit int, offset int) ([]*UserPreferences, error) {
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

	var records []*UserPreferences
	if err := userPreferencesChangesQuery(db, uid, startSnapshotID, endSnapshotID).
		Order("GREATEST(updated_at, COALESCE(deleted_at, updated_at)) ASC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func UserPreferencesToPB(record *UserPreferences) *pb.UserPreferences {
	if record == nil {
		return nil
	}

	return &pb.UserPreferences{
		Id:                        record.ID,
		Uid:                       record.UID,
		Theme:                     record.Theme,
		ThemeOpacity:              record.ThemeOpacity,
		CalendarLightPalette:      record.CalendarLightPalette,
		CalendarDarkPalette:       record.CalendarDarkPalette,
		HidesNavigationTitleSpark: record.HidesNavigationTitleSpark,
		SchemaVersion:             record.SchemaVersion,
		CreatedAt:                 record.CreatedAt.UnixMilli(),
		UpdatedAt:                 record.UpdatedAt.UnixMilli(),
	}
}

func normalizeUserPreferences(uid uint64, prefs *pb.UserPreferences) *UserPreferences {
	theme := strings.TrimSpace(prefs.GetTheme())
	if theme == "" {
		theme = DefaultUserPreferencesTheme
	}

	opacity := prefs.GetThemeOpacity()
	if opacity < 45 {
		opacity = 45
	}
	if opacity > 100 {
		opacity = 100
	}

	lightPalette := strings.TrimSpace(prefs.GetCalendarLightPalette())
	if lightPalette == "" {
		lightPalette = DefaultUserPreferencesCalendarPalette
	}

	darkPalette := strings.TrimSpace(prefs.GetCalendarDarkPalette())
	if darkPalette == "" {
		darkPalette = DefaultUserPreferencesCalendarPalette
	}

	schemaVersion := prefs.GetSchemaVersion()
	if schemaVersion <= 0 {
		schemaVersion = DefaultUserPreferencesSchemaVersion
	}

	return &UserPreferences{
		UID:                       uid,
		Theme:                     theme,
		ThemeOpacity:              opacity,
		CalendarLightPalette:      lightPalette,
		CalendarDarkPalette:       darkPalette,
		HidesNavigationTitleSpark: prefs.GetHidesNavigationTitleSpark(),
		SchemaVersion:             schemaVersion,
	}
}

func userPreferencesChangesQuery(db *gorm.DB, uid uint64, startSnapshotID int64, endSnapshotID int64) *gorm.DB {
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
