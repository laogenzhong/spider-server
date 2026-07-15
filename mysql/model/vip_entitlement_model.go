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

	UserEntitlementSourceApple      = "apple"
	UserEntitlementSourceAdminGrant = "admin_grant"

	ApplePurchaseOrderStatusCreated = "created"
	ApplePurchaseOrderStatusPaid    = "paid"
	ApplePurchaseOrderStatusExpired = "expired"

	ApplePurchaseOrderSourcePrePurchase   = "pre_purchase"
	ApplePurchaseOrderSourcePostLoginBind = "post_login_bind"

	AppStoreNotificationStatusProcessed   = "processed"
	AppStoreNotificationStatusPendingUser = "pending_user"
	AppStoreNotificationStatusIgnored     = "ignored"

	AppStoreSubscriptionStatusActive             int32 = 1
	AppStoreSubscriptionStatusExpired            int32 = 2
	AppStoreSubscriptionStatusBillingRetry       int32 = 3
	AppStoreSubscriptionStatusBillingGracePeriod int32 = 4
	AppStoreSubscriptionStatusRevoked            int32 = 5

	applePurchaseOrderTransactionClockSkew = 5 * time.Minute
)

var (
	ErrApplePurchaseOrderNotFound            = errors.New("apple purchase order not found")
	ErrApplePurchaseOrderExpired             = errors.New("apple purchase order expired")
	ErrApplePurchaseOrderProductMismatch     = errors.New("apple purchase order product mismatch")
	ErrApplePurchaseOrderTransactionMismatch = errors.New("apple purchase order transaction mismatch")
	ErrAppleTransactionOwnedByOtherUser      = errors.New("apple transaction owned by other user")
	ErrAdminVIPAccountNotFound               = errors.New("admin vip account not found")
	ErrAdminVIPDurationInvalid               = errors.New("admin vip duration invalid")
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
	AdminGranted          bool   `gorm:"index;not null;default:false"`
	AdminGrantKind        string `gorm:"size:32"`
	AdminGrantExpiresAt   *time.Time
	AdminGrantOperator    string `gorm:"size:64"`
	AdminGrantReason      string `gorm:"size:255"`
	AdminGrantedAt        *time.Time
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
	AppAccountToken       string `gorm:"size:64;index"`
	OfferIdentifier       string `gorm:"size:128;index"`
	OfferType             int32  `gorm:"index;not null;default:0"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

type AppleTransactionOwnership struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	UID                   uint64 `gorm:"index;not null"`
	OriginalTransactionID string `gorm:"size:128;uniqueIndex;not null"`
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
	Source                string `gorm:"size:32;index;not null;default:pre_purchase"`
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
	OfferIdentifier       string `gorm:"size:128;index"`
	OfferType             int32  `gorm:"index;not null;default:0"`
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

type AppleTransactionReconcileRef struct {
	UID                   uint64
	TransactionID         string
	OriginalTransactionID string
	ProductID             string
	UpdatedAt             time.Time
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
		if strings.TrimSpace(order.ProductID) != strings.TrimSpace(tx.ProductID) {
			return ErrApplePurchaseOrderProductMismatch
		}
		if order.Status == ApplePurchaseOrderStatusPaid &&
			strings.TrimSpace(order.TransactionID) != "" &&
			strings.TrimSpace(order.TransactionID) != strings.TrimSpace(tx.TransactionID) {
			return ErrApplePurchaseOrderProductMismatch
		}
		confirmationSource, err := applePurchaseOrderConfirmationSource(order, record, now)
		if err != nil {
			return err
		}
		record.OrderID = order.OrderID

		if err := upsertAppleTransaction(db, record, true); err != nil {
			return err
		}

		if err := markApplePurchaseOrderPaid(db, order, record.TransactionID, record.OriginalTransactionID, confirmationSource, now); err != nil {
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
		Source:    ApplePurchaseOrderSourcePrePurchase,
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

	return currentVIPStatusFromEntitlement(entitlement, now), nil
}

func GetUserByAdminVIPIdentifier(identifier string) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("identifier is empty")
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}
	return findUserByAdminVIPIdentifier(db, identifier, false)
}

func GrantAdminVIPByAccount(
	account string,
	lifetime bool,
	durationDays int64,
	expiresAtUnix int64,
	operator string,
	reason string,
	now time.Time,
) (*User, CurrentVIPStatus, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, CurrentVIPStatus{}, fmt.Errorf("account is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}

	kind := VIPKindMonthly
	var expiresAt *time.Time
	if lifetime {
		kind = VIPKindLifetime
	} else {
		switch {
		case expiresAtUnix > 0:
			t := time.Unix(expiresAtUnix, 0)
			expiresAt = &t
		case durationDays > 0:
			t := now.Add(time.Duration(durationDays) * 24 * time.Hour)
			expiresAt = &t
		default:
			return nil, CurrentVIPStatus{}, ErrAdminVIPDurationInvalid
		}
		if expiresAt == nil || !expiresAt.After(now) {
			return nil, CurrentVIPStatus{}, ErrAdminVIPDurationInvalid
		}
	}

	operator = truncateString(strings.TrimSpace(operator), 64)
	reason = truncateString(strings.TrimSpace(reason), 255)

	var user *User
	var status CurrentVIPStatus
	err := config.WithTx(func(db *gorm.DB) error {
		foundUser, err := findUserByAdminVIPIdentifier(db, account, true)
		if err != nil {
			return err
		}
		user = foundUser

		entitlement := &UserEntitlement{}
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uid = ? AND entitlement = ?", uint64(foundUser.ID), UserEntitlementVIP).
			First(entitlement).Error
		if err != nil {
			if !isRecordNotFound(err) {
				return err
			}
			entitlement = &UserEntitlement{
				UID:         uint64(foundUser.ID),
				Entitlement: UserEntitlementVIP,
				Kind:        VIPKindNone,
				Active:      false,
			}
		}

		entitlement.AdminGranted = true
		entitlement.AdminGrantKind = kind
		entitlement.AdminGrantExpiresAt = expiresAt
		entitlement.AdminGrantOperator = operator
		entitlement.AdminGrantReason = reason
		entitlement.AdminGrantedAt = &now
		if entitlement.ID == 0 {
			if err := db.Create(entitlement).Error; err != nil {
				return err
			}
		} else if err := db.Save(entitlement).Error; err != nil {
			return err
		}

		status = currentVIPStatusFromEntitlement(entitlement, now)
		return nil
	})
	if err != nil {
		return nil, CurrentVIPStatus{}, err
	}

	return user, status, nil
}

func RevokeAdminVIPByAccount(account string, operator string, reason string, now time.Time) (*User, CurrentVIPStatus, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, CurrentVIPStatus{}, fmt.Errorf("account is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}

	operator = truncateString(strings.TrimSpace(operator), 64)
	reason = truncateString(strings.TrimSpace(reason), 255)

	var user *User
	var status CurrentVIPStatus
	err := config.WithTx(func(db *gorm.DB) error {
		foundUser, err := findUserByAdminVIPIdentifier(db, account, true)
		if err != nil {
			return err
		}
		user = foundUser

		entitlement := &UserEntitlement{}
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uid = ? AND entitlement = ?", uint64(foundUser.ID), UserEntitlementVIP).
			First(entitlement).Error
		if err != nil {
			if isRecordNotFound(err) {
				status = CurrentVIPStatus{Kind: VIPKindNone}
				return nil
			}
			return err
		}

		entitlement.AdminGranted = false
		entitlement.AdminGrantKind = VIPKindNone
		entitlement.AdminGrantExpiresAt = nil
		entitlement.AdminGrantOperator = operator
		entitlement.AdminGrantReason = reason
		entitlement.AdminGrantedAt = nil
		if err := db.Save(entitlement).Error; err != nil {
			return err
		}

		status = currentVIPStatusFromEntitlement(entitlement, now)
		return nil
	})
	if err != nil {
		return nil, CurrentVIPStatus{}, err
	}

	return user, status, nil
}

func findUserByAdminVIPIdentifier(db *gorm.DB, identifier string, forUpdate bool) (*User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrAdminVIPAccountNotFound
	}

	userQuery := db
	if forUpdate {
		userQuery = userQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	user := &User{}
	err := userQuery.Where("account = ?", identifier).First(user).Error
	if err == nil {
		return user, nil
	}
	if !isRecordNotFound(err) {
		return nil, err
	}

	friendUserID := strings.ToUpper(identifier)
	profile := &FriendProfileRecord{}
	err = db.Where("user_id = ?", friendUserID).First(profile).Error
	if err == nil {
		return findUserByID(db, profile.UID, forUpdate)
	}
	if !isRecordNotFound(err) {
		return nil, err
	}

	if uid, ok := parseDefaultFriendUserID(friendUserID); ok {
		return findUserByID(db, uid, forUpdate)
	}
	return nil, ErrAdminVIPAccountNotFound
}

func findUserByID(db *gorm.DB, uid uint64, forUpdate bool) (*User, error) {
	if uid == 0 {
		return nil, ErrAdminVIPAccountNotFound
	}

	query := db
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	user := &User{}
	if err := query.Where("id = ?", uid).First(user).Error; err != nil {
		if isRecordNotFound(err) {
			return nil, ErrAdminVIPAccountNotFound
		}
		return nil, err
	}
	return user, nil
}

func ListAppleTransactionsForAppStoreReconcile(limit int) ([]AppleTransactionReconcileRef, error) {
	if limit <= 0 {
		limit = 50
	}

	db, err := config.DB()
	if err != nil {
		return nil, err
	}

	var transactions []AppleTransaction
	if err := db.Where("uid > 0 AND transaction_id <> ''").
		Order("updated_at ASC, id ASC").
		Limit(limit * 3).
		Find(&transactions).Error; err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(transactions))
	refs := make([]AppleTransactionReconcileRef, 0, limit)
	for _, transaction := range transactions {
		transactionID := strings.TrimSpace(transaction.TransactionID)
		originalTransactionID := strings.TrimSpace(transaction.OriginalTransactionID)
		reconcileTransactionID := firstNonEmpty(originalTransactionID, transactionID)
		if transaction.UID == 0 || reconcileTransactionID == "" {
			continue
		}

		key := fmt.Sprintf("%d:%s", transaction.UID, reconcileTransactionID)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		refs = append(refs, AppleTransactionReconcileRef{
			UID:                   transaction.UID,
			TransactionID:         transactionID,
			OriginalTransactionID: originalTransactionID,
			ProductID:             strings.TrimSpace(transaction.ProductID),
			UpdatedAt:             transaction.UpdatedAt,
		})
		if len(refs) >= limit {
			break
		}
	}

	return refs, nil
}

func ApplyAppStoreServerAPITransactions(
	uid uint64,
	transactions []appstore.VerifiedTransaction,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if len(transactions) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	return config.WithTx(func(db *gorm.DB) error {
		for _, verifiedTransaction := range transactions {
			transaction := verifiedTransaction.Transaction
			if strings.TrimSpace(transaction.TransactionID) == "" {
				continue
			}

			kind := vipKindForProduct(transaction.ProductID, monthlyProductID, lifetimeProductID)
			if kind == VIPKindNone {
				continue
			}

			record := appleTransactionFromVerifiedPayload(uid, transaction, verifiedTransaction.SignedTransactionJWS)
			record.OriginalTransactionID = firstNonEmpty(record.OriginalTransactionID, record.TransactionID)
			if err := upsertAppleTransaction(db, record, false); err != nil {
				return err
			}
			if err := applyAppleTransactionRecordForReconcile(db, record, kind, now); err != nil {
				return err
			}
			if err := applyPendingAppStoreNotificationsForOriginalTransaction(db, uid, record.OriginalTransactionID, monthlyProductID, lifetimeProductID, now); err != nil {
				return err
			}
		}

		return nil
	})
}

func ApplyAppStoreSubscriptionStatuses(
	uid uint64,
	items []appstore.SubscriptionStatusItem,
	monthlyProductID string,
	lifetimeProductID string,
	now time.Time,
) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	if len(items) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	return config.WithTx(func(db *gorm.DB) error {
		for _, item := range items {
			if item.Transaction == nil || strings.TrimSpace(item.Transaction.TransactionID) == "" {
				continue
			}

			transaction := *item.Transaction
			kind := vipKindForProduct(transaction.ProductID, monthlyProductID, lifetimeProductID)
			if kind == VIPKindNone {
				continue
			}

			record := appleTransactionFromVerifiedPayload(uid, transaction, item.SignedTransactionJWS)
			record.OriginalTransactionID = firstNonEmpty(record.OriginalTransactionID, item.OriginalTransactionID, record.TransactionID)
			if err := upsertAppleTransaction(db, record, false); err != nil {
				return err
			}
			if err := applySubscriptionStatusItemForReconcile(db, record, item, kind, now); err != nil {
				return err
			}
			if err := applyPendingAppStoreNotificationsForOriginalTransaction(db, uid, record.OriginalTransactionID, monthlyProductID, lifetimeProductID, now); err != nil {
				return err
			}
		}

		return nil
	})
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
			if err := upsertAppStoreServerNotification(db, record); err != nil {
				return err
			}
			createApplePaymentFailureInTxBestEffort(db, ApplePaymentFailure{
				Category:              ApplePaymentFailureCategoryPendingUser,
				Stage:                 ApplePaymentFailureStagePendingUserMatch,
				Severity:              ApplePaymentFailureSeverityWarning,
				UID:                   uid,
				ProductID:             record.ProductID,
				TransactionID:         record.TransactionID,
				OriginalTransactionID: record.OriginalTransactionID,
				NotificationUUID:      record.NotificationUUID,
				NotificationType:      record.NotificationType,
				Subtype:               record.Subtype,
				BundleID:              record.BundleID,
				Environment:           record.Environment,
				Reason:                record.ProcessingError,
				Problem:               "App Store notification could not be matched to a local uid, so the entitlement change is waiting for a later client transaction confirmation.",
				ContextJSON: ApplePaymentFailureContext(map[string]any{
					"subscriptionStatus":    record.SubscriptionStatus,
					"autoRenewProductID":    record.AutoRenewProductID,
					"autoRenewStatus":       record.AutoRenewStatus,
					"expirationIntent":      record.ExpirationIntent,
					"isInBillingRetry":      record.IsInBillingRetry,
					"gracePeriodExpiresAt":  record.GracePeriodExpiresAt,
					"notificationSignedAt":  record.NotificationSignedAt,
					"transactionSignedAt":   record.TransactionSignedAt,
					"signedPayloadLength":   len(strings.TrimSpace(record.SignedPayload)),
					"signedTransactionDots": strings.Count(strings.TrimSpace(record.SignedTransactionJWS), "."),
				}),
				OccurredAt: now,
			})
			return nil
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

func applyAppleTransactionRecordForReconcile(db *gorm.DB, record *AppleTransaction, kind string, now time.Time) error {
	if record == nil || record.UID == 0 {
		return nil
	}
	originalTransactionID := firstNonEmpty(record.OriginalTransactionID, record.TransactionID)
	if originalTransactionID == "" {
		return nil
	}

	if kind == VIPKindLifetime {
		if record.RevocationAt != nil {
			return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, record.ExpiresAt, now)
		}
		return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, nil)
	}

	if record.RevocationAt != nil {
		return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, record.ExpiresAt, now)
	}
	if isVIPEntitlementCurrentlyActive(kind, record.ExpiresAt, now) {
		return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, record.ExpiresAt)
	}
	return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, record.ExpiresAt, now)
}

func applySubscriptionStatusItemForReconcile(
	db *gorm.DB,
	record *AppleTransaction,
	item appstore.SubscriptionStatusItem,
	kind string,
	now time.Time,
) error {
	if record == nil || record.UID == 0 {
		return nil
	}
	originalTransactionID := firstNonEmpty(record.OriginalTransactionID, item.OriginalTransactionID, record.TransactionID)
	if originalTransactionID == "" {
		return nil
	}

	if kind == VIPKindLifetime {
		return applyAppleTransactionRecordForReconcile(db, record, kind, now)
	}

	expiresAt := record.ExpiresAt
	if item.RenewalInfo != nil && item.RenewalInfo.GracePeriodExpiresDate > 0 {
		expiresAt = laterTimePtr(expiresAt, millisToTimePtr(item.RenewalInfo.GracePeriodExpiresDate))
	}

	switch item.Status {
	case AppStoreSubscriptionStatusActive:
		if isVIPEntitlementCurrentlyActive(kind, expiresAt, now) {
			return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, expiresAt)
		}
		return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, expiresAt, now)
	case AppStoreSubscriptionStatusBillingRetry, AppStoreSubscriptionStatusBillingGracePeriod:
		if isVIPEntitlementCurrentlyActive(kind, expiresAt, now) {
			return upsertVIPEntitlement(db, record.UID, kind, record.ProductID, originalTransactionID, expiresAt)
		}
		return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, expiresAt, now)
	case AppStoreSubscriptionStatusExpired, AppStoreSubscriptionStatusRevoked:
		return deactivateMatchingVIPEntitlement(db, record.UID, record.ProductID, originalTransactionID, expiresAt, now)
	default:
		return applyAppleTransactionRecordForReconcile(db, record, kind, now)
	}
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
		record.OfferIdentifier = strings.TrimSpace(transaction.OfferIdentifier)
		record.OfferType = transaction.OfferType
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

	ownership := &AppleTransactionOwnership{}
	err = db.Where("original_transaction_id IN ?", ids).
		Order("updated_at DESC, id DESC").
		First(ownership).Error
	if err == nil && ownership.UID != 0 {
		return ownership.UID, nil
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
	if err := ensureAppleTransactionOwnership(db, record); err != nil {
		return err
	}

	columns := []string{
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
		"app_account_token",
		"offer_identifier",
		"offer_type",
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

func ensureAppleTransactionOwnership(db *gorm.DB, record *AppleTransaction) error {
	if record == nil || record.UID == 0 {
		return nil
	}

	transactionID := strings.TrimSpace(record.TransactionID)
	originalTransactionID := firstNonEmpty(record.OriginalTransactionID, transactionID)
	if transactionID == "" && originalTransactionID == "" {
		return nil
	}
	if err := claimAppleTransactionOwnership(db, record.UID, originalTransactionID); err != nil {
		return err
	}

	var existing AppleTransaction
	query := db.Clauses(clause.Locking{Strength: "UPDATE"})
	if transactionID != "" && originalTransactionID != "" {
		query = query.Where("transaction_id = ? OR original_transaction_id = ?", transactionID, originalTransactionID)
	} else if transactionID != "" {
		query = query.Where("transaction_id = ?", transactionID)
	} else {
		query = query.Where("original_transaction_id = ?", originalTransactionID)
	}
	err := query.Order("updated_at DESC, id DESC").First(&existing).Error
	if err == nil {
		if existing.UID != 0 && existing.UID != record.UID {
			return ErrAppleTransactionOwnedByOtherUser
		}
		return nil
	}
	if err != nil && !isRecordNotFound(err) {
		return err
	}

	var existingEntitlement UserEntitlement
	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("entitlement = ? AND original_transaction_id = ?", UserEntitlementVIP, originalTransactionID).
		Order("updated_at DESC, id DESC").
		First(&existingEntitlement).Error
	if err == nil {
		if existingEntitlement.UID != 0 && existingEntitlement.UID != record.UID {
			return ErrAppleTransactionOwnedByOtherUser
		}
		return nil
	}
	if err != nil && !isRecordNotFound(err) {
		return err
	}

	return nil
}

func claimAppleTransactionOwnership(db *gorm.DB, uid uint64, originalTransactionID string) error {
	originalTransactionID = strings.TrimSpace(originalTransactionID)
	if uid == 0 || originalTransactionID == "" {
		return nil
	}

	ownership := &AppleTransactionOwnership{
		UID:                   uid,
		OriginalTransactionID: originalTransactionID,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "original_transaction_id"}},
		DoNothing: true,
	}).Create(ownership).Error; err != nil {
		return err
	}

	existing := &AppleTransactionOwnership{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("original_transaction_id = ?", originalTransactionID).
		First(existing).Error
	if err != nil {
		return err
	}
	if existing.UID != 0 && existing.UID != uid {
		return ErrAppleTransactionOwnedByOtherUser
	}
	return nil
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
			"offer_identifier",
			"offer_type",
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
		existing.Source = UserEntitlementSourceApple
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
		Source:                UserEntitlementSourceApple,
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
		AppAccountToken:       strings.TrimSpace(tx.AppAccountToken),
		OfferIdentifier:       strings.TrimSpace(tx.OfferIdentifier),
		OfferType:             tx.OfferType,
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
		OfferIdentifier:       strings.TrimSpace(notification.OfferIdentifier),
		OfferType:             notification.OfferType,
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

func markApplePurchaseOrderPaid(db *gorm.DB, order *ApplePurchaseOrder, transactionID string, originalTransactionID string, source string, now time.Time) error {
	order.Status = ApplePurchaseOrderStatusPaid
	order.Source = normalizeApplePurchaseOrderSource(source)
	order.TransactionID = transactionID
	order.OriginalTransactionID = originalTransactionID
	order.ConfirmedAt = &now
	return db.Save(order).Error
}

func applePurchaseOrderConfirmationSource(order *ApplePurchaseOrder, record *AppleTransaction, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now()
	}
	if order == nil || record == nil || record.PurchaseAt == nil {
		return "", ErrApplePurchaseOrderTransactionMismatch
	}
	if order.Status != ApplePurchaseOrderStatusPaid && order.ExpiresAt.Before(now) {
		return "", ErrApplePurchaseOrderExpired
	}

	if validateApplePurchaseOrderTransactionWindow(order, record) == nil {
		return ApplePurchaseOrderSourcePrePurchase, nil
	}

	if isPostLoginBindApplePurchaseOrder(order, record) {
		return ApplePurchaseOrderSourcePostLoginBind, nil
	}

	return "", ErrApplePurchaseOrderTransactionMismatch
}

func validateApplePurchaseOrderTransactionWindow(order *ApplePurchaseOrder, record *AppleTransaction) error {
	if order == nil || record == nil || record.PurchaseAt == nil {
		return ErrApplePurchaseOrderTransactionMismatch
	}

	if strings.TrimSpace(record.AppAccountToken) == "" ||
		strings.TrimSpace(record.AppAccountToken) != strings.TrimSpace(order.OrderID) {
		return ErrApplePurchaseOrderTransactionMismatch
	}

	earliest := order.CreatedAt.Add(-applePurchaseOrderTransactionClockSkew)
	latest := order.ExpiresAt.Add(applePurchaseOrderTransactionClockSkew)
	if record.PurchaseAt.Before(earliest) || record.PurchaseAt.After(latest) {
		return ErrApplePurchaseOrderTransactionMismatch
	}
	return nil
}

func isPostLoginBindApplePurchaseOrder(order *ApplePurchaseOrder, record *AppleTransaction) bool {
	if order == nil || record == nil || record.PurchaseAt == nil {
		return false
	}
	if strings.TrimSpace(record.TransactionID) == "" || strings.TrimSpace(record.OriginalTransactionID) == "" {
		return false
	}
	if strings.TrimSpace(order.ProductID) != strings.TrimSpace(record.ProductID) {
		return false
	}

	token := strings.TrimSpace(record.AppAccountToken)
	if token == "" || token != strings.TrimSpace(order.OrderID) {
		return true
	}

	earliest := order.CreatedAt.Add(-applePurchaseOrderTransactionClockSkew)
	latest := order.ExpiresAt.Add(applePurchaseOrderTransactionClockSkew)
	return record.PurchaseAt.Before(earliest) || record.PurchaseAt.After(latest)
}

func normalizeApplePurchaseOrderSource(source string) string {
	source = strings.TrimSpace(source)
	switch source {
	case ApplePurchaseOrderSourcePostLoginBind:
		return ApplePurchaseOrderSourcePostLoginBind
	default:
		return ApplePurchaseOrderSourcePrePurchase
	}
}

func generateApplePurchaseOrderID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(bytes)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	), nil
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

func currentVIPStatusFromEntitlement(entitlement *UserEntitlement, now time.Time) CurrentVIPStatus {
	if entitlement == nil {
		return CurrentVIPStatus{Kind: VIPKindNone}
	}

	apple := CurrentVIPStatus{
		Kind:      normalizeVIPKind(entitlement.Kind),
		ProductID: entitlement.ProductID,
		Source:    entitlement.Source,
		ExpiresAt: entitlement.ExpiresAt,
	}
	apple.IsVIP = entitlement.Active && isVIPEntitlementCurrentlyActive(apple.Kind, apple.ExpiresAt, now)
	if !apple.IsVIP {
		apple.Kind = VIPKindNone
	}

	admin := CurrentVIPStatus{
		Kind:      normalizeVIPKind(entitlement.AdminGrantKind),
		Source:    UserEntitlementSourceAdminGrant,
		ExpiresAt: entitlement.AdminGrantExpiresAt,
	}
	admin.IsVIP = entitlement.AdminGranted && isVIPEntitlementCurrentlyActive(admin.Kind, admin.ExpiresAt, now)
	if !admin.IsVIP {
		admin.Kind = VIPKindNone
	}

	return preferredVIPStatus(apple, admin)
}

func preferredVIPStatus(a CurrentVIPStatus, b CurrentVIPStatus) CurrentVIPStatus {
	if !a.IsVIP {
		if b.IsVIP {
			return b
		}
		return a
	}
	if !b.IsVIP {
		return a
	}
	if a.Kind == VIPKindLifetime {
		return a
	}
	if b.Kind == VIPKindLifetime {
		return b
	}
	if laterTimePtr(a.ExpiresAt, b.ExpiresAt) == b.ExpiresAt {
		return b
	}
	return a
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

func truncateString(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
