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

		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "transaction_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"uid",
				"order_id",
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
			}),
		}).Create(record).Error; err != nil {
			return err
		}

		if err := markApplePurchaseOrderPaid(db, order, record.TransactionID, record.OriginalTransactionID, now); err != nil {
			return err
		}

		active := record.RevocationAt == nil && isVIPEntitlementCurrentlyActive(kind, record.ExpiresAt, now)
		if !active {
			return deactivateMatchingVIPEntitlement(db, uid, record.OriginalTransactionID)
		}

		return upsertVIPEntitlement(db, uid, kind, record.ProductID, record.OriginalTransactionID, record.ExpiresAt)
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

func upsertVIPEntitlement(db *gorm.DB, uid uint64, kind string, productID string, originalTransactionID string, expiresAt *time.Time) error {
	existing := &UserEntitlement{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("uid = ? AND entitlement = ?", uid, UserEntitlementVIP).
		First(existing).Error
	if err == nil {
		existingActive := existing.Active && isVIPEntitlementCurrentlyActive(existing.Kind, existing.ExpiresAt, time.Now())
		if existingActive && normalizeVIPKind(existing.Kind) == VIPKindLifetime && kind == VIPKindMonthly {
			return nil
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

func deactivateMatchingVIPEntitlement(db *gorm.DB, uid uint64, originalTransactionID string) error {
	if strings.TrimSpace(originalTransactionID) == "" {
		return nil
	}
	return db.Model(&UserEntitlement{}).
		Where("uid = ? AND entitlement = ? AND original_transaction_id = ?", uid, UserEntitlementVIP, originalTransactionID).
		Updates(map[string]any{
			"active": false,
			"kind":   VIPKindNone,
		}).Error
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
