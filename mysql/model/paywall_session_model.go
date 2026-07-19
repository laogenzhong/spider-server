package mysqlmodel

import (
	"fmt"
	"strings"
	"time"

	"spider-server/mysql/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaywallSessionStatusDefault   = "default"
	PaywallSessionStatusCancelled = "cancelled"
	PaywallSessionStatusPurchased = "purchased"
)

type PaywallSessionRecord struct {
	ID              uint      `gorm:"primaryKey;autoIncrement"`
	PresentationID  string    `gorm:"size:64;uniqueIndex;not null"`
	UID             uint64    `gorm:"index;not null;default:0"`
	AnonymousID     string    `gorm:"size:64;index"`
	DeviceUniqueID  string    `gorm:"size:64;index"`
	EntryPoint      string    `gorm:"size:64;index;not null"`
	PresentedAt     time.Time `gorm:"index;not null"`
	Status          string    `gorm:"size:16;index;not null;default:default"`
	StatusChangedAt time.Time `gorm:"index;not null"`
	ProductID       string    `gorm:"size:128;index"`
	AppVersion      string    `gorm:"size:32;index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PaywallSessionWrite struct {
	PresentationID  string
	UID             uint64
	AnonymousID     string
	DeviceUniqueID  string
	EntryPoint      string
	PresentedAt     time.Time
	Status          string
	StatusChangedAt time.Time
	ProductID       string
	AppVersion      string
}

func RecordPaywallSession(write PaywallSessionWrite) error {
	write.PresentationID = truncateString(strings.TrimSpace(write.PresentationID), 64)
	write.AnonymousID = truncateString(strings.TrimSpace(write.AnonymousID), 64)
	write.DeviceUniqueID = truncateString(strings.TrimSpace(write.DeviceUniqueID), 64)
	write.EntryPoint = truncateString(strings.TrimSpace(write.EntryPoint), 64)
	write.ProductID = truncateString(strings.TrimSpace(write.ProductID), 128)
	write.AppVersion = truncateString(strings.TrimSpace(write.AppVersion), 32)
	write.Status = normalizePaywallSessionStatus(write.Status)
	if write.PresentationID == "" {
		return fmt.Errorf("presentation id is empty")
	}
	if write.EntryPoint == "" {
		return fmt.Errorf("entry point is empty")
	}
	if write.PresentedAt.IsZero() {
		return fmt.Errorf("presented at is empty")
	}
	if write.StatusChangedAt.IsZero() {
		write.StatusChangedAt = write.PresentedAt
	}

	return config.WithTx(func(db *gorm.DB) error {
		record := &PaywallSessionRecord{}
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("presentation_id = ?", write.PresentationID).
			First(record).Error
		if err != nil {
			if !isRecordNotFound(err) {
				return err
			}
			return db.Create(&PaywallSessionRecord{
				PresentationID:  write.PresentationID,
				UID:             write.UID,
				AnonymousID:     write.AnonymousID,
				DeviceUniqueID:  write.DeviceUniqueID,
				EntryPoint:      write.EntryPoint,
				PresentedAt:     write.PresentedAt,
				Status:          write.Status,
				StatusChangedAt: write.StatusChangedAt,
				ProductID:       write.ProductID,
				AppVersion:      write.AppVersion,
			}).Error
		}

		if paywallSessionStatusRank(write.Status) < paywallSessionStatusRank(record.Status) {
			return nil
		}
		if paywallSessionStatusRank(write.Status) == paywallSessionStatusRank(record.Status) &&
			!write.StatusChangedAt.After(record.StatusChangedAt) {
			return nil
		}
		record.Status = write.Status
		record.StatusChangedAt = write.StatusChangedAt
		if write.ProductID != "" {
			record.ProductID = write.ProductID
		}
		if record.AppVersion == "" {
			record.AppVersion = write.AppVersion
		}
		if record.DeviceUniqueID == "" {
			record.DeviceUniqueID = write.DeviceUniqueID
		}
		return db.Save(record).Error
	})
}

func normalizePaywallSessionStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case PaywallSessionStatusPurchased:
		return PaywallSessionStatusPurchased
	case PaywallSessionStatusCancelled:
		return PaywallSessionStatusCancelled
	default:
		return PaywallSessionStatusDefault
	}
}

func paywallSessionStatusRank(status string) int {
	switch normalizePaywallSessionStatus(status) {
	case PaywallSessionStatusPurchased:
		return 2
	case PaywallSessionStatusCancelled:
		return 1
	default:
		return 0
	}
}

// markExistingPaywallSessionPurchased is a best-effort correlation fallback for
// the narrow window where Apple confirmation succeeds before the client can
// enqueue its purchased event. Missing sessions are intentionally left alone;
// the persisted client queue remains the source of the original presented_at.
func markExistingPaywallSessionPurchased(db *gorm.DB, presentationID string, productID string, purchasedAt time.Time) error {
	presentationID = truncateString(strings.TrimSpace(presentationID), 64)
	if presentationID == "" || db == nil {
		return nil
	}
	if purchasedAt.IsZero() {
		purchasedAt = time.Now()
	}
	return db.Model(&PaywallSessionRecord{}).
		Where("presentation_id = ?", presentationID).
		Where("status <> ? OR status_changed_at < ?", PaywallSessionStatusPurchased, purchasedAt).
		Updates(map[string]any{
			"status":            PaywallSessionStatusPurchased,
			"status_changed_at": purchasedAt,
			"product_id":        truncateString(strings.TrimSpace(productID), 128),
		}).Error
}
