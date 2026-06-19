package mysqlmodel

import (
	"errors"
	appconfig "spider-server/common/config"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const AppUpdatePlatformIOS = "ios"

type AppUpdateConfig struct {
	ID                     uint64 `gorm:"primaryKey;autoIncrement"`
	Platform               string `gorm:"size:32;not null;uniqueIndex"`
	LatestVersion          string `gorm:"size:32"`
	MinSupportedVersion    string `gorm:"size:32"`
	ForceUpdateEnabled     bool
	UpdateAvailableEnabled bool
	AppStoreURL            string `gorm:"size:512"`
	MessageZhHans          string `gorm:"type:text"`
	MessageZhHant          string `gorm:"type:text"`
	MessageEn              string `gorm:"type:text"`
	MessageJa              string `gorm:"type:text"`
	MessageKo              string `gorm:"type:text"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              gorm.DeletedAt `gorm:"index"`
}

type AppUpdateConfigInput struct {
	Platform               string
	LatestVersion          string
	MinSupportedVersion    string
	ForceUpdateEnabled     bool
	UpdateAvailableEnabled bool
	AppStoreURL            string
	MessageZhHans          string
	MessageZhHant          string
	MessageEn              string
	MessageJa              string
	MessageKo              string
}

func GetAppUpdateConfig(platform string) (*AppUpdateConfig, error) {
	platform = normalizeAppUpdatePlatform(platform)
	record := &AppUpdateConfig{}
	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Where("platform = ?", platform).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func UpsertAppUpdateConfig(input AppUpdateConfigInput) (*AppUpdateConfig, error) {
	platform := normalizeAppUpdatePlatform(input.Platform)
	record := &AppUpdateConfig{
		Platform:               platform,
		LatestVersion:          strings.TrimSpace(input.LatestVersion),
		MinSupportedVersion:    strings.TrimSpace(input.MinSupportedVersion),
		ForceUpdateEnabled:     input.ForceUpdateEnabled,
		UpdateAvailableEnabled: input.UpdateAvailableEnabled,
		AppStoreURL:            strings.TrimSpace(input.AppStoreURL),
		MessageZhHans:          strings.TrimSpace(input.MessageZhHans),
		MessageZhHant:          strings.TrimSpace(input.MessageZhHant),
		MessageEn:              strings.TrimSpace(input.MessageEn),
		MessageJa:              strings.TrimSpace(input.MessageJa),
		MessageKo:              strings.TrimSpace(input.MessageKo),
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "platform"}},
		DoUpdates: clause.Assignments(map[string]any{
			"latest_version":           record.LatestVersion,
			"min_supported_version":    record.MinSupportedVersion,
			"force_update_enabled":     record.ForceUpdateEnabled,
			"update_available_enabled": record.UpdateAvailableEnabled,
			"app_store_url":            record.AppStoreURL,
			"message_zh_hans":          record.MessageZhHans,
			"message_zh_hant":          record.MessageZhHant,
			"message_en":               record.MessageEn,
			"message_ja":               record.MessageJa,
			"message_ko":               record.MessageKo,
			"deleted_at":               nil,
			"updated_at":               time.Now(),
		}),
	}).Create(record).Error; err != nil {
		return nil, err
	}
	return GetAppUpdateConfig(platform)
}

func CreateAppUpdateConfigIfMissing(input AppUpdateConfigInput) (*AppUpdateConfig, bool, error) {
	platform := normalizeAppUpdatePlatform(input.Platform)
	existing, err := GetAppUpdateConfig(platform)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	created, err := UpsertAppUpdateConfig(input)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func SeedAppUpdateConfigFromAppConfig(cfg appconfig.AppUpdateConfig) error {
	_, _, err := CreateAppUpdateConfigIfMissing(AppUpdateConfigInput{
		Platform:               AppUpdatePlatformIOS,
		LatestVersion:          cfg.IOSLatestVersion,
		MinSupportedVersion:    cfg.IOSMinSupportedVersion,
		ForceUpdateEnabled:     cfg.ForceUpdateEnabled,
		UpdateAvailableEnabled: cfg.UpdateAvailableEnabled,
		AppStoreURL:            cfg.IOSAppStoreURL,
		MessageZhHans:          cfg.MessageZhHans,
		MessageZhHant:          cfg.MessageZhHant,
		MessageEn:              cfg.MessageEn,
		MessageJa:              cfg.MessageJa,
		MessageKo:              cfg.MessageKo,
	})
	return err
}

func (c *AppUpdateConfig) MessageForLanguage(systemLanguage string) string {
	lang := strings.ToLower(strings.TrimSpace(systemLanguage))
	switch {
	case strings.HasPrefix(lang, "zh-hant"), strings.Contains(lang, "hant"), strings.HasPrefix(lang, "zh-tw"), strings.HasPrefix(lang, "zh-hk"):
		return strings.TrimSpace(c.MessageZhHant)
	case strings.HasPrefix(lang, "ja"):
		return strings.TrimSpace(c.MessageJa)
	case strings.HasPrefix(lang, "ko"):
		return strings.TrimSpace(c.MessageKo)
	case strings.HasPrefix(lang, "en"):
		return strings.TrimSpace(c.MessageEn)
	case strings.HasPrefix(lang, "zh"):
		return strings.TrimSpace(c.MessageZhHans)
	default:
		if message := strings.TrimSpace(c.MessageEn); message != "" {
			return message
		}
		return strings.TrimSpace(c.MessageZhHans)
	}
}

func normalizeAppUpdatePlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return AppUpdatePlatformIOS
	}
	return platform
}
