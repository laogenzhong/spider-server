package mysqlmodel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"spider-server/game/appstore"
	"spider-server/mysql/config"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	UserEntitlementVIP = "vip"

	VIPKindNone     = "none"
	VIPKindLifetime = "lifetime"
	VIPKindMonthly  = "monthly"

	ApplePurchaseOrderStatusCreated = "created"
	ApplePurchaseOrderStatusPaid    = "paid"
	ApplePurchaseOrderStatusExpired = "expired"

	AppStoreNotificationStatusProcessed   = "processed"
	AppStoreNotificationStatusPendingUser = "pending_user"
	AppStoreNotificationStatusIgnored     = "ignored"
)

var (
	ErrApplePurchaseOrderNotFound        = errors.New("apple purchase order not found")
	ErrApplePurchaseOrderExpired         = errors.New("apple purchase order expired")
	ErrApplePurchaseOrderProductMismatch = errors.New("apple purchase order product mismatch")
)

type UserEntitlement struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	UID                   uint64 `gorm:"uniqueIndex:idx_user_entitlement_uid_name;not null"`
	Entitlement           string `gorm:"size:32;uniqueIndex:idx_user_entitlement_uid_name;not null"`
	Kind                  string `gorm:"size:32;index;not null"`
	Active                bool   `gorm:"index;not null;default:false"`
	ExpiresAt             *time.Time
	ProductID             string `gorm:"size:128"`
	OriginalTransactionID string `gorm:"size:128;index"`
	Source                string `gorm:"size:32"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

type AppleTransaction struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	UID                   uint64 `gorm:"index;not null"`
	OrderID               string `gorm:"size:64;index"`
	TransactionID         string `gorm:"size:128;uniqueIndex;not null"`
	OriginalTransactionID string `gorm:"size:128;index"`
	ProductID             string `gorm:"size:128;index;not null"`
	BundleID              string `gorm:"size:128;index"`
	Environment           string `gorm:"size:32;index"`
	Type                  string `gorm:"size:64"`
	PurchaseAt            *time.Time
	OriginalPurchaseAt    *time.Time
	ExpiresAt             *time.Time
	RevocationAt          *time.Time
	RevocationReason      int32
	SignedAt              *time.Time
	SignedTransactionJWS  string `gorm:"type:text"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

type ApplePurchaseOrder struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	UID                   uint64 `gorm:"index;not null"`
	OrderID               string `gorm:"size:64;uniqueIndex;not null"`
	ProductID             string `gorm:"size:128;index;not null"`
	Status                string `gorm:"size:32;index;not null"`
	TransactionID         string `gorm:"size:128;index"`
	OriginalTransactionID string `gorm:"size:128;index"`
	ExpiresAt             time.Time
	ConfirmedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

type AppStoreServerNotification struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	UID                   uint64 `gorm:"index;not null;default:0"`
	NotificationUUID      string `gorm:"size:128;uniqueIndex;not null"`
	NotificationType      string `gorm:"size:64;index"`
	Subtype               string `gorm:"size:64;index"`
	Version               string `gorm:"size:16"`
	BundleID              string `gorm:"size:128;index"`
	Environment           string `gorm:"size:32;index"`
	AppAppleID            int64
	BundleVersion         string `gorm:"size:64"`
	SubscriptionStatus    int32
	ConsumptionReason     string `gorm:"size:64"`
	ProductID             string `gorm:"size:128;index"`
	TransactionID         string `gorm:"size:128;index"`
	OriginalTransactionID string `gorm:"size:128;index"`
	TransactionType       string `gorm:"size:64"`
	PurchaseAt            *time.Time
	OriginalPurchaseAt    *time.Time
	ExpiresAt             *time.Time
	RevocationAt          *time.Time
	RevocationReason      int32
	TransactionSignedAt   *time.Time
	AutoRenewProductID    string `gorm:"size:128"`
	AutoRenewStatus       int32
	ExpirationIntent      int32
	IsInBillingRetry      bool `gorm:"index;not null;default:false"`
	GracePeriodExpiresAt  *time.Time
	RenewalDate           *time.Time
	RenewalSignedAt       *time.Time
	NotificationSignedAt  *time.Time
	SignedPayload         string `gorm:"type:text"`
	SignedTransactionJWS  string `gorm:"type:text"`
	SignedRenewalInfoJWS  string `gorm:"type:text"`
	ProcessingStatus      string `gorm:"size:32;index;not null"`
	ProcessingError       string `gorm:"type:text"`
	ProcessedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

type CurrentVIPStatus struct {
	IsVIP     bool
	Kind      string
	ExpiresAt *time.Time
	ProductID string
	Source    string
}

func SaveAppleTransactionAndGrantVIP(
	uid uint64,
	orderID string,
	tx appstore.Transaction,
	signedTransactionJWS string,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if strings.TrimSpace(tx.TransactionID) == "" {
		return fmt.Errorf("transaction id is empty")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return ErrApplePurchaseOrderNotFound
	}
	if now.IsZero() {
		now = time.Now()
	}

	kind := vipKindForProduct(tx.ProductID, monthlyProductID, lifetimeProductID)
	if kind == VIPKindNone {
		return fmt.Errorf("unsupported product id: %s", tx.ProductID)
	}

	record := appleTransactionFromVerifiedPayload(uid, tx, signedTransactionJWS)
	record.OrderID = orderID
	return config.WithTx(func(db *gorm.DB) error {
		order, err := lockApplePurchaseOrder(db, uid, orderID)
		if err != nil {
			return err
		}
		if order.ExpiresAt.Before(now) && order.Status != ApplePurchaseOrderStatusPaid {
			if updateErr := markApplePurchaseOrderExpired(db, order); updateErr != nil {
				return updateErr
			}
			return ErrApplePurchaseOrderExpired
		}
		if strings.TrimSpace(order.ProductID) != strings.TrimSpace(tx.ProductID) {
			return ErrApplePurchaseOrderProductMismatch
		}
		if order.Status == ApplePurchaseOrderStatusPaid &&
			strings.TrimSpace(order.TransactionID) != "" &&
			strings.TrimSpace(order.TransactionID) != strings.TrimSpace(tx.TransactionID) {
			return ErrApplePurchaseOrderProductMismatch
		}

		if err := upsertAppleTransaction(db, record, true); err != nil {
			return err
		}

		if err := markApplePurchaseOrderPaid(db, order, record.TransactionID, record.OriginalTransactionID, now); err != nil {
			return err
		}

		active := record.RevocationAt == nil && isVIPEntitlementCurrentlyActive(kind, record.ExpiresAt, now)
		if !active {
			if err := deactivateMatchingVIPEntitlement(db, uid, record.ProductID, record.OriginalTransactionID, record.ExpiresAt, now); err != nil {
				return err
			}
			return applyPendingAppStoreNotificationsForOriginalTransaction(db, uid, record.OriginalTransactionID, monthlyProductID, lifetimeProductID, now)
		}

		if err := upsertVIPEntitlement(db, uid, kind, record.ProductID, record.OriginalTransactionID, record.ExpiresAt); err != nil {
			return err
		}
		return applyPendingAppStoreNotificationsForOriginalTransaction(db, uid, record.OriginalTransactionID, monthlyProductID, lifetimeProductID, now)
	})
}

func CreateApplePurchaseOrder(uid uint64, productID string, monthlyProductID string, lifetimeProductID string, now time.Time) (*ApplePurchaseOrder, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is empty")
	}
	if vipKindForProduct(productID, monthlyProductID, lifetimeProductID) == VIPKindNone {
		return nil, fmt.Errorf("unsupported product id: %s", productID)
	}
	if now.IsZero() {
		now = time.Now()
	}

	orderID, err := generateApplePurchaseOrderID()
	if err != nil {
		return nil, err
	}
	order := &ApplePurchaseOrder{
		UID:       uid,
		OrderID:   orderID,
		ProductID: productID,
		Status:    ApplePurchaseOrderStatusCreated,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	return order, config.Create(order)
}

func GetCurrentVIPStatus(uid uint64, now time.Time) (CurrentVIPStatus, error) {
	if uid == 0 {
		return CurrentVIPStatus{}, fmt.Errorf("uid is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}

	entitlement := &UserEntitlement{}
	err := config.First(entitlement, "uid = ? AND entitlement = ?", uid, UserEntitlementVIP)
	if err != nil {
		if isRecordNotFound(err) {
			return CurrentVIPStatus{Kind: VIPKindNone}, nil
		}
		return CurrentVIPStatus{}, err
	}

	status := CurrentVIPStatus{
		Kind:      normalizeVIPKind(entitlement.Kind),
		ProductID: entitlement.ProductID,
		Source:    entitlement.Source,
		ExpiresAt: entitlement.ExpiresAt,
	}
	status.IsVIP = entitlement.Active && isVIPEntitlementCurrentlyActive(status.Kind, status.ExpiresAt, now)
	if !status.IsVIP {
		status.Kind = VIPKindNone
	}

	return status, nil
}

func SaveAppStoreServerNotificationAndApplyVIP(
	notification appstore.Notification,
	transaction *appstore.Transaction,
	renewalInfo *appstore.RenewalInfo,
	signedPayload string,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	if strings.TrimSpace(notification.NotificationUUID) == "" {
		return fmt.Errorf("notification uuid is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}

	record := appStoreServerNotificationFromPayload(notification, transaction, renewalInfo, signedPayload)
	return config.WithTx(func(db *gorm.DB) error {
		existing := &AppStoreServerNotification{}
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("notification_uuid = ?", record.NotificationUUID).
			First(existing).Error
		if err == nil &&
			(existing.ProcessingStatus == AppStoreNotificationStatusProcessed ||
				existing.ProcessingStatus == AppStoreNotificationStatusIgnored) {
			return nil
		}
		if err != nil && !isRecordNotFound(err) {
			return err
		}

		uid, err := findVIPOwnerUIDForNotification(db, transaction, record.OriginalTransactionID)
		if err != nil {
			return err
		}
		record.UID = uid

		if transaction == nil || strings.TrimSpace(record.TransactionID) == "" {
			record.ProcessingStatus = AppStoreNotificationStatusIgnored
			record.ProcessingError = "notification has no transaction"
			record.ProcessedAt = &now
			return upsertAppStoreServerNotification(db, record)
		}

		if vipKindForProduct(record.ProductID, monthlyProductID, lifetimeProductID) == VIPKindNone {
			record.ProcessingStatus = AppStoreNotificationStatusIgnored
			record.ProcessingError = "unsupported product id"
			record.ProcessedAt = &now
			return upsertAppStoreServerNotification(db, record)
		}

		if uid == 0 {
			record.ProcessingStatus = AppStoreNotificationStatusPendingUser
			record.ProcessingError = "original transaction owner not found"
			return upsertAppStoreServerNotification(db, record)
		}

		return applyAppStoreServerNotificationRecord(db, record, monthlyProductID, lifetimeProductID, now)
	})
}

func applyPendingAppStoreNotificationsForOriginalTransaction(
	db *gorm.DB,
	uid uint64,
	originalTransactionID string,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if uid == 0 || originalTransactionID == "" {
		return nil
	}

	var records []AppStoreServerNotification
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("original_transaction_id = ? AND processing_status = ?", originalTransactionID, AppStoreNotificationStatusPendingUser).
		Order("notification_signed_at ASC, id ASC").
		Find(&records).Error
	if err != nil {
		return err
	}

	for i := range records {
		records[i].UID = uid
		if err := applyAppStoreServerNotificationRecord(db, &records[i], monthlyProductID, lifetimeProductID, now); err != nil {
			return err
		}
	}

	return nil
}

func applyAppStoreServerNotificationRecord(
	db *gorm.DB,
	record *AppStoreServerNotification,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	if now.IsZero() {
		now = time.Now()
	}

	kind := vipKindForProduct(record.ProductID, monthlyProductID, lifetimeProductID)
	if kind == VIPKindNone {
		record.ProcessingStatus = AppStoreNotificationStatusIgnored
		record.ProcessingError = "unsupported product id"
		record.ProcessedAt = &now
		return upsertAppStoreServerNotification(db, record)
	}

	transaction := appleTransactionFromNotificationRecord(record.UID, record)
	if transaction != nil {
		if err := upsertAppleTransaction(db, transaction, false); err != nil {
			return err
		}
	}

	if err := applyVIPEntitlementForAppStoreNotification(db, record, kind, now); err != nil {
		return err
	}

	record.ProcessingStatus = AppStoreNotificationStatusProcessed
	record.ProcessingError = ""
	record.ProcessedAt = &now
	return upsertAppStoreServerNotification(db, record)
}

func applyVIPEntitlementForAppStoreNotification(db *gorm.DB, record *AppStoreServerNotification, kind string, now time.Time) error {
	originalTransactionID := strings.TrimSpace(record.OriginalTransactionID)
	if originalTransactionID == "" {
		originalTransactionID = strings.TrimSpace(record.TransactionID)
	}
	if record.UID == 0 || originalTransactionID == "" {
		return nil
	}

	notificationType := normalizeNotificationName(record.NotificationType)
	if kind == VIPKindLifetime {
		if isRevocationNotification(notificationType) || record.RevocationAt != nil {
			return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, record.ExpiresAt, now)
		}
		return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, nil)
	}

	if isRevocationNotification(notificationType) || record.RevocationAt != nil || isExpirationNotification(notificationType) {
		return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, record.ExpiresAt, now)
	}

	expiresAt := record.ExpiresAt
	if shouldUseGracePeriod(record, now) {
		expiresAt = laterTimePtr(expiresAt, record.GracePeriodExpiresAt)
	}

	if isVIPEntitlementCurrentlyActive(kind, expiresAt, now) {
		return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, expiresAt)
	}

	if isEntitlementNeutralNotification(notificationType) {
		return nil
	}

	return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, expiresAt, now)
}

func appStoreServerNotificationFromPayload(
	notification appstore.Notification,
	transaction *appstore.Transaction,
	renewalInfo *appstore.RenewalInfo,
	signedPayload string,
) *AppStoreServerNotification {
	data := notification.Data
	record := &AppStoreServerNotification{
		NotificationUUID:     strings.TrimSpace(notification.NotificationUUID),
		NotificationType:     normalizeNotificationName(notification.NotificationType),
		Subtype:              normalizeNotificationName(notification.Subtype),
		Version:              strings.TrimSpace(notification.Version),
		BundleID:             strings.TrimSpace(data.BundleID),
		Environment:          normalizeNotificationName(data.Environment),
		AppAppleID:           data.AppAppleID,
		BundleVersion:        strings.TrimSpace(data.BundleVersion),
		SubscriptionStatus:   data.Status,
		ConsumptionReason:    strings.TrimSpace(data.ConsumptionReason),
		NotificationSignedAt: millisToTimePtr(notification.SignedDate),
		SignedPayload:        strings.TrimSpace(signedPayload),
		SignedTransactionJWS: strings.TrimSpace(data.SignedTransactionInfo),
		SignedRenewalInfoJWS: strings.TrimSpace(data.SignedRenewalInfo),
		ProcessingStatus:     AppStoreNotificationStatusPendingUser,
	}

	if transaction != nil {
		record.TransactionID = strings.TrimSpace(transaction.TransactionID)
		record.OriginalTransactionID = strings.TrimSpace(transaction.OriginalTransactionID)
		record.ProductID = strings.TrimSpace(transaction.ProductID)
		record.BundleID = firstNonEmpty(record.BundleID, transaction.BundleID)
		record.Environment = firstNonEmpty(record.Environment, normalizeNotificationName(transaction.Environment))
		record.TransactionType = strings.TrimSpace(transaction.Type)
		record.PurchaseAt = millisToTimePtr(transaction.PurchaseDate)
		record.OriginalPurchaseAt = millisToTimePtr(transaction.OriginalPurchaseDate)
		record.ExpiresAt = millisToTimePtr(transaction.ExpiresDate)
		record.RevocationAt = millisToTimePtr(transaction.RevocationDate)
		record.RevocationReason = transaction.RevocationReason
		record.TransactionSignedAt = millisToTimePtr(transaction.SignedDate)
	}

	if renewalInfo != nil {
		record.OriginalTransactionID = firstNonEmpty(record.OriginalTransactionID, renewalInfo.OriginalTransactionID)
		record.ProductID = firstNonEmpty(record.ProductID, renewalInfo.ProductID)
		record.Environment = firstNonEmpty(record.Environment, normalizeNotificationName(renewalInfo.Environment))
		record.AutoRenewProductID = strings.TrimSpace(renewalInfo.AutoRenewProductID)
		record.AutoRenewStatus = renewalInfo.AutoRenewStatus
		record.ExpirationIntent = renewalInfo.ExpirationIntent
		record.IsInBillingRetry = renewalInfo.IsInBillingRetryPeriod
		record.GracePeriodExpiresAt = millisToTimePtr(renewalInfo.GracePeriodExpiresDate)
		record.RenewalDate = millisToTimePtr(renewalInfo.RenewalDate)
		record.RenewalSignedAt = millisToTimePtr(renewalInfo.SignedDate)
	}

	return record
}

func findVIPOwnerUIDForNotification(db *gorm.DB, transaction *appstore.Transaction, originalTransactionID string) (uint64, error) {
	ids := normalizedTransactionIDs(originalTransactionID)
	if transaction != nil {
		ids = append(ids, normalizedTransactionIDs(transaction.OriginalTransactionID, transaction.TransactionID)...)
	}
	ids = uniqueNonEmptyStrings(ids)
	if len(ids) == 0 {
		return 0, nil
	}

	appleTransaction := &AppleTransaction{}
	err := db.Where("original_transaction_id IN ? OR transaction_id IN ?", ids, ids).
		Order("updated_at DESC, id DESC").
		First(appleTransaction).Error
	if err == nil && appleTransaction.UID != 0 {
		return appleTransaction.UID, nil
	}
	if err != nil && !isRecordNotFound(err) {
		return 0, err
	}

	order := &ApplePurchaseOrder{}
	err = db.Where("(original_transaction_id IN ? OR transaction_id IN ?) AND status = ?", ids, ids, ApplePurchaseOrderStatusPaid).
		Order("updated_at DESC, id DESC").
		First(order).Error
	if err == nil && order.UID != 0 {
		return order.UID, nil
	}
	if err != nil && !isRecordNotFound(err) {
		return 0, err
	}

	entitlement := &UserEntitlement{}
	err = db.Where("entitlement = ? AND original_transaction_id IN ?", UserEntitlementVIP, ids).
		Order("updated_at DESC, id DESC").
		First(entitlement).Error
	if err == nil && entitlement.UID != 0 {
		return entitlement.UID, nil
	}
	if err != nil && !isRecordNotFound(err) {
		return 0, err
	}

	return 0, nil
}

func upsertAppleTransaction(db *gorm.DB, record *AppleTransaction, updateOrderID bool) error {
	columns := []string{
		"uid",
		"original_transaction_id",
		"product_id",
		"bundle_id",
		"environment",
		"type",
		"purchase_at",
		"original_purchase_at",
		"expires_at",
		"revocation_at",
		"revocation_reason",
		"signed_at",
		"signed_transaction_jws",
		"updated_at",
	}
	if updateOrderID {
		columns = append(columns, "order_id")
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "transaction_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(record).Error
}

func upsertAppStoreServerNotification(db *gorm.DB, record *AppStoreServerNotification) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "notification_uuid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"uid",
			"notification_type",
			"subtype",
			"version",
			"bundle_id",
			"environment",
			"app_apple_id",
			"bundle_version",
			"subscription_status",
			"consumption_reason",
			"product_id",
			"transaction_id",
			"original_transaction_id",
			"transaction_type",
			"purchase_at",
			"original_purchase_at",
			"expires_at",
			"revocation_at",
			"revocation_reason",
			"transaction_signed_at",
			"auto_renew_product_id",
			"auto_renew_status",
			"expiration_intent",
			"is_in_billing_retry",
			"grace_period_expires_at",
			"renewal_date",
			"renewal_signed_at",
			"notification_signed_at",
			"signed_payload",
			"signed_transaction_jws",
			"signed_renewal_info_jws",
			"processing_status",
			"processing_error",
			"processed_at",
			"updated_at",
		}),
	}).Create(record).Error
}

func upsertVIPEntitlement(db *gorm.DB, uid uint64, kind string, productID string, originalTransactionID string, expiresAt *time.Time) error {
	kind = normalizeVIPKind(kind)
	productID = strings.TrimSpace(productID)
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if kind == VIPKindMonthly {
		latestExpiresAt, err := latestAppleTransactionExpiresAt(db, uid, originalTransactionID)
		if err != nil {
			return err
		}
		expiresAt = laterTimePtr(expiresAt, latestExpiresAt)
	}

	existing := &UserEntitlement{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uid = ? AND entitlement = ?", uid, UserEntitlementVIP).
		First(existing).Error
	if err == nil {
		existingActive := existing.Active && isVIPEntitlementCurrentlyActive(existing.Kind, existing.ExpiresAt, time.Now())
		if existingActive && normalizeVIPKind(existing.Kind) == VIPKindLifetime && kind == VIPKindMonthly {
			return nil
		}
		if kind == VIPKindMonthly {
			expiresAt = laterTimePtr(existing.ExpiresAt, expiresAt)
		}
		existing.Kind = kind
		existing.Active = true
		existing.ExpiresAt = expiresAt
		existing.ProductID = productID
		existing.OriginalTransactionID = originalTransactionID
		existing.Source = "apple"
		return db.Save(existing).Error
	}
	if err != nil && !isRecordNotFound(err) {
		return err
	}

	entitlement := &UserEntitlement{
		UID:                   uid,
		Entitlement:           UserEntitlementVIP,
		Kind:                  kind,
		Active:                true,
		ExpiresAt:             expiresAt,
		ProductID:             productID,
		OriginalTransactionID: originalTransactionID,
		Source:                "apple",
	}
	return db.Create(entitlement).Error
}

func deactivateMatchingVIPEntitlement(db *gorm.DB, uid uint64, productID string, originalTransactionID string, expiresAt *time.Time, now time.Time) error {
	productID = strings.TrimSpace(productID)
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if uid == 0 || originalTransactionID == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	latestActiveTransaction, err := latestActiveAppleTransactionForOriginalTransaction(db, uid, originalTransactionID, now)
	if err != nil {
		return err
	}
	if latestActiveTransaction != nil {
		return upsertVIPEntitlement(
			db,
			uid,
			VIPKindMonthly,
			firstNonEmpty(latestActiveTransaction.ProductID, productID),
			originalTransactionID,
			laterTimePtr(expiresAt, latestActiveTransaction.ExpiresAt),
		)
	}

	latestExpiresAt, err := latestAppleTransactionExpiresAt(db, uid, originalTransactionID)
	if err != nil {
		return err
	}
	expiresAt = laterTimePtr(expiresAt, latestExpiresAt)

	updates := map[string]any{
		"active": false,
		"kind":   VIPKindNone,
	}
	if expiresAt != nil {
		updates["expires_at"] = expiresAt
	}
	if productID != "" {
		updates["product_id"] = productID
	}
	return db.Model(&UserEntitlement{}).
		Where("uid = ? AND entitlement = ? AND original_transaction_id = ?", uid, UserEntitlementVIP, originalTransactionID).
		Updates(updates).Error
}

func appleTransactionFromVerifiedPayload(uid uint64, tx appstore.Transaction, signedTransactionJWS string) *AppleTransaction {
	return &AppleTransaction{
		UID:                   uid,
		TransactionID:         tx.TransactionID,
		OriginalTransactionID: tx.OriginalTransactionID,
		ProductID:             tx.ProductID,
		BundleID:              tx.BundleID,
		Environment:           tx.Environment,
		Type:                  tx.Type,
		PurchaseAt:            millisToTimePtr(tx.PurchaseDate),
		OriginalPurchaseAt:    millisToTimePtr(tx.OriginalPurchaseDate),
		ExpiresAt:             millisToTimePtr(tx.ExpiresDate),
		RevocationAt:          millisToTimePtr(tx.RevocationDate),
		RevocationReason:      tx.RevocationReason,
		SignedAt:              millisToTimePtr(tx.SignedDate),
		SignedTransactionJWS:  strings.TrimSpace(signedTransactionJWS),
	}
}

func appleTransactionFromNotificationRecord(uid uint64, notification *AppStoreServerNotification) *AppleTransaction {
	if uid == 0 || strings.TrimSpace(notification.TransactionID) == "" {
		return nil
	}
	return &AppleTransaction{
		UID:                   uid,
		TransactionID:         strings.TrimSpace(notification.TransactionID),
		OriginalTransactionID: strings.TrimSpace(notification.OriginalTransactionID),
		ProductID:             strings.TrimSpace(notification.ProductID),
		BundleID:              strings.TrimSpace(notification.BundleID),
		Environment:           strings.TrimSpace(notification.Environment),
		Type:                  strings.TrimSpace(notification.TransactionType),
		PurchaseAt:            notification.PurchaseAt,
		OriginalPurchaseAt:    notification.OriginalPurchaseAt,
		ExpiresAt:             notification.ExpiresAt,
		RevocationAt:          notification.RevocationAt,
		RevocationReason:      notification.RevocationReason,
		SignedAt:              notification.TransactionSignedAt,
		SignedTransactionJWS:  strings.TrimSpace(notification.SignedTransactionJWS),
	}
}

func normalizeNotificationName(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func isRevocationNotification(notificationType string) bool {
	switch normalizeNotificationName(notificationType) {
	case "REFUND", "REVOKE":
		return true
	default:
		return false
	}
}

func isExpirationNotification(notificationType string) bool {
	switch normalizeNotificationName(notificationType) {
	case "EXPIRED", "GRACE_PERIOD_EXPIRED":
		return true
	default:
		return false
	}
}

func isEntitlementNeutralNotification(notificationType string) bool {
	switch normalizeNotificationName(notificationType) {
	case "CONSUMPTION_REQUEST",
		"DID_CHANGE_RENEWAL_PREF",
		"DID_CHANGE_RENEWAL_STATUS",
		"PRICE_INCREASE",
		"REFUND_DECLINED",
		"TEST":
		return true
	default:
		return false
	}
}

func shouldUseGracePeriod(notification *AppStoreServerNotification, now time.Time) bool {
	if notification == nil || notification.GracePeriodExpiresAt == nil {
		return false
	}
	if notification.GracePeriodExpiresAt.Before(now) {
		return false
	}
	notificationType := normalizeNotificationName(notification.NotificationType)
	subtype := normalizeNotificationName(notification.Subtype)
	return notificationType == "DID_FAIL_TO_RENEW" || subtype == "GRACE_PERIOD" || notification.IsInBillingRetry
}

func laterTimePtr(a *time.Time, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.After(*a) {
		return b
	}
	return a
}

func latestAppleTransactionExpiresAt(db *gorm.DB, uid uint64, originalTransactionID string) (*time.Time, error) {
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if uid == 0 || originalTransactionID == "" {
		return nil, nil
	}

	transaction := &AppleTransaction{}
	err := db.Where(
		"uid = ? AND (original_transaction_id = ? OR transaction_id = ?) AND expires_at IS NOT NULL",
		uid,
		originalTransactionID,
		originalTransactionID,
	).
		Order("expires_at DESC, id DESC").
		First(transaction).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return transaction.ExpiresAt, nil
}

func latestActiveAppleTransactionForOriginalTransaction(db *gorm.DB, uid uint64, originalTransactionID string, now time.Time) (*AppleTransaction, error) {
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if uid == 0 || originalTransactionID == "" {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	transaction := &AppleTransaction{}
	err := db.Where(
		"uid = ? AND (original_transaction_id = ? OR transaction_id = ?) AND revocation_at IS NULL AND expires_at IS NOT NULL AND expires_at > ?",
		uid,
		originalTransactionID,
		originalTransactionID,
		now,
	).
		Order("expires_at DESC, id DESC").
		First(transaction).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return transaction, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizedTransactionIDs(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func lockApplePurchaseOrder(db *gorm.DB, uid uint64, orderID string) (*ApplePurchaseOrder, error) {
	order := &ApplePurchaseOrder{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uid = ? AND order_id = ?", uid, orderID).
		First(order).Error
	if err != nil {
		if isRecordNotFound(err) {
			return nil, ErrApplePurchaseOrderNotFound
		}
		return nil, err
	}
	return order, nil
}

func markApplePurchaseOrderExpired(db *gorm.DB, order *ApplePurchaseOrder) error {
	order.Status = ApplePurchaseOrderStatusExpired
	return db.Save(order).Error
}

func markApplePurchaseOrderPaid(db *gorm.DB, order *ApplePurchaseOrder, transactionID string, originalTransactionID string, now time.Time) error {
	order.Status = ApplePurchaseOrderStatusPaid
	order.TransactionID = transactionID
	order.OriginalTransactionID = originalTransactionID
	order.ConfirmedAt = &now
	return db.Save(order).Error
}

func generateApplePurchaseOrderID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "iap_" + hex.EncodeToString(bytes), nil
}

func vipKindForProduct(productID string, monthlyProductID string, lifetimeProductID string) string {
	productID = strings.TrimSpace(productID)
	switch productID {
	case strings.TrimSpace(lifetimeProductID):
		return VIPKindLifetime
	case strings.TrimSpace(monthlyProductID):
		return VIPKindMonthly
	default:
		return VIPKindNone
	}
}

func millisToTimePtr(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.UnixMilli(value)
	return &t
}

func isVIPEntitlementCurrentlyActive(kind string, expiresAt *time.Time, now time.Time) bool {
	switch normalizeVIPKind(kind) {
	case VIPKindLifetime:
		return true
	case VIPKindMonthly:
		return expiresAt != nil && expiresAt.After(now)
	default:
		return false
	}
}

func normalizeVIPKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case VIPKindLifetime:
		return VIPKindLifetime
	case VIPKindMonthly:
		return VIPKindMonthly
	default:
		return VIPKindNone
	}
}
