package mysqlmodel

import (
	"encoding/json"
	"fmt"
	applogger "spider-server/common/logger"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ApplePaymentFailureSeverityWarning  = "warning"
	ApplePaymentFailureSeverityCritical = "critical"

	ApplePaymentFailureStatusOpen     = "open"
	ApplePaymentFailureStatusResolved = "resolved"

	ApplePaymentFailureCategoryTransactionVerify  = "transaction_verify_failed"
	ApplePaymentFailureCategoryNotificationVerify = "notification_verify_failed"
	ApplePaymentFailureCategoryNotification5xx    = "notification_5xx"
	ApplePaymentFailureCategoryPendingUser        = "pending_user"
	ApplePaymentFailureCategoryPendingUserBacklog = "pending_user_backlog"
	ApplePaymentFailureCategoryRefundRevoke       = "refund_revoke_failed"
	ApplePaymentFailureCategoryReconcile          = "reconcile_failed"

	ApplePaymentFailureStageTransactionVerify  = "transaction_verify"
	ApplePaymentFailureStageNotificationVerify = "notification_verify"
	ApplePaymentFailureStageNotificationApply  = "notification_apply"
	ApplePaymentFailureStagePendingUserMatch   = "pending_user_match"
	ApplePaymentFailureStagePendingUserBacklog = "pending_user_backlog"
	ApplePaymentFailureStageRefundRevokeApply  = "refund_revoke_apply"
	ApplePaymentFailureStageReconcile          = "reconcile"
)

type ApplePaymentFailure struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement"`
	Category              string    `gorm:"size:64;index;not null"`
	Stage                 string    `gorm:"size:64;index;not null"`
	Severity              string    `gorm:"size:16;index;not null"`
	Status                string    `gorm:"size:16;index;not null;default:open"`
	UID                   uint64    `gorm:"index;not null;default:0"`
	OrderID               string    `gorm:"size:64;index"`
	ProductID             string    `gorm:"size:128;index"`
	TransactionID         string    `gorm:"size:128;index"`
	OriginalTransactionID string    `gorm:"size:128;index"`
	NotificationUUID      string    `gorm:"size:128;index"`
	NotificationType      string    `gorm:"size:64;index"`
	Subtype               string    `gorm:"size:64;index"`
	BundleID              string    `gorm:"size:128;index"`
	Environment           string    `gorm:"size:32;index"`
	HTTPStatus            int       `gorm:"index;not null;default:0"`
	ErrorCode             int       `gorm:"index;not null;default:0"`
	Reason                string    `gorm:"type:text"`
	Problem               string    `gorm:"type:text"`
	ErrorMessage          string    `gorm:"type:text"`
	ContextJSON           string    `gorm:"type:text"`
	OccurredAt            time.Time `gorm:"index;not null"`
	Alerted               bool      `gorm:"index;not null;default:false"`
	AlertedAt             *time.Time
	ResolvedAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

func RecordApplePaymentFailureBestEffort(record ApplePaymentFailure) {
	if err := RecordApplePaymentFailure(record); err != nil {
		applogger.Printf("record apple payment failure failed: category=%s stage=%s err=%v", record.Category, record.Stage, err)
	}
}

func RecordApplePaymentFailure(record ApplePaymentFailure) error {
	normalizeApplePaymentFailure(&record)
	return config.Create(&record)
}

func ApplePaymentFailureContext(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func RecordPendingAppStoreNotificationBacklog(now time.Time) error {
	if now.IsZero() {
		now = time.Now()
	}
	db, err := config.DB()
	if err != nil {
		return err
	}

	var count int64
	if err := db.Model(&AppStoreServerNotification{}).
		Where("processing_status = ?", AppStoreNotificationStatusPendingUser).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	var oldest AppStoreServerNotification
	if err := db.Where("processing_status = ?", AppStoreNotificationStatusPendingUser).
		Order("created_at ASC, id ASC").
		First(&oldest).Error; err != nil && !isRecordNotFound(err) {
		return err
	}

	var latest AppStoreServerNotification
	if err := db.Where("processing_status = ?", AppStoreNotificationStatusPendingUser).
		Order("created_at DESC, id DESC").
		First(&latest).Error; err != nil && !isRecordNotFound(err) {
		return err
	}

	severity := ApplePaymentFailureSeverityWarning
	if count >= 20 {
		severity = ApplePaymentFailureSeverityCritical
	}
	return RecordApplePaymentFailure(ApplePaymentFailure{
		Category:              ApplePaymentFailureCategoryPendingUserBacklog,
		Stage:                 ApplePaymentFailureStagePendingUserBacklog,
		Severity:              severity,
		UID:                   latest.UID,
		ProductID:             latest.ProductID,
		TransactionID:         latest.TransactionID,
		OriginalTransactionID: latest.OriginalTransactionID,
		NotificationUUID:      latest.NotificationUUID,
		NotificationType:      latest.NotificationType,
		Subtype:               latest.Subtype,
		BundleID:              latest.BundleID,
		Environment:           latest.Environment,
		Reason:                fmt.Sprintf("pending_user_count=%d", count),
		Problem:               "App Store notifications are accumulating in pending_user status because the original transaction owner cannot be matched yet.",
		ContextJSON: ApplePaymentFailureContext(map[string]any{
			"pendingUserCount": count,
			"oldest": map[string]any{
				"notificationUUID":      oldest.NotificationUUID,
				"notificationType":      oldest.NotificationType,
				"transactionID":         oldest.TransactionID,
				"originalTransactionID": oldest.OriginalTransactionID,
				"createdAt":             oldest.CreatedAt,
			},
			"latest": map[string]any{
				"notificationUUID":      latest.NotificationUUID,
				"notificationType":      latest.NotificationType,
				"transactionID":         latest.TransactionID,
				"originalTransactionID": latest.OriginalTransactionID,
				"createdAt":             latest.CreatedAt,
			},
		}),
		OccurredAt: now,
	})
}

func createApplePaymentFailureInTxBestEffort(db *gorm.DB, record ApplePaymentFailure) {
	normalizeApplePaymentFailure(&record)
	if err := db.Create(&record).Error; err != nil {
		applogger.Printf("record apple payment failure in tx failed: category=%s stage=%s err=%v", record.Category, record.Stage, err)
	}
}

func normalizeApplePaymentFailure(record *ApplePaymentFailure) {
	record.Category = strings.TrimSpace(record.Category)
	record.Stage = strings.TrimSpace(record.Stage)
	record.Severity = strings.TrimSpace(record.Severity)
	record.Status = strings.TrimSpace(record.Status)
	if record.Category == "" {
		record.Category = "unknown"
	}
	if record.Stage == "" {
		record.Stage = "unknown"
	}
	if record.Severity == "" {
		record.Severity = ApplePaymentFailureSeverityWarning
	}
	if record.Status == "" {
		record.Status = ApplePaymentFailureStatusOpen
	}
	if record.OccurredAt.IsZero() {
		record.OccurredAt = time.Now()
	}

	record.OrderID = strings.TrimSpace(record.OrderID)
	record.ProductID = strings.TrimSpace(record.ProductID)
	record.TransactionID = strings.TrimSpace(record.TransactionID)
	record.OriginalTransactionID = strings.TrimSpace(record.OriginalTransactionID)
	record.NotificationUUID = strings.TrimSpace(record.NotificationUUID)
	record.NotificationType = normalizeNotificationName(record.NotificationType)
	record.Subtype = normalizeNotificationName(record.Subtype)
	record.BundleID = strings.TrimSpace(record.BundleID)
	record.Environment = normalizeNotificationName(record.Environment)
	record.Reason = strings.TrimSpace(record.Reason)
	record.Problem = strings.TrimSpace(record.Problem)
	record.ErrorMessage = strings.TrimSpace(record.ErrorMessage)
	record.ContextJSON = strings.TrimSpace(record.ContextJSON)
}
