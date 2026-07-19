package mysqlmodel

import (
	"errors"
	"spider-server/game/appstore"
	"testing"
	"time"
)

func TestParseAdminVIPUID(t *testing.T) {
	tests := []struct {
		input string
		uid   uint64
		ok    bool
	}{
		{input: "29", uid: 29, ok: true},
		{input: " 29 ", uid: 29, ok: true},
		{input: "SP000029", uid: 0, ok: false},
		{input: "0", uid: 0, ok: false},
		{input: "account29", uid: 0, ok: false},
	}

	for _, test := range tests {
		uid, ok := parseAdminVIPUID(test.input)
		if uid != test.uid || ok != test.ok {
			t.Fatalf("parseAdminVIPUID(%q) = (%d, %v), want (%d, %v)", test.input, uid, ok, test.uid, test.ok)
		}
	}
}

func TestApplePurchaseOrderConfirmationSourcePrePurchase(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	order := &ApplePurchaseOrder{
		OrderID:   "11111111-2222-4333-8444-555555555555",
		ProductID: "hh.spider.vip.monthly",
		Status:    ApplePurchaseOrderStatusCreated,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	record := &AppleTransaction{
		TransactionID:         "tx-1",
		OriginalTransactionID: "original-1",
		ProductID:             order.ProductID,
		PurchaseAt:            timePtr(now.Add(2 * time.Minute)),
		AppAccountToken:       order.OrderID,
	}

	source, err := applePurchaseOrderConfirmationSource(order, record, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("confirmation source returned error: %v", err)
	}
	if source != ApplePurchaseOrderSourcePrePurchase {
		t.Fatalf("source = %q, want %q", source, ApplePurchaseOrderSourcePrePurchase)
	}
}

func TestApplePurchaseOrderConfirmationSourcePostLoginBind(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	order := &ApplePurchaseOrder{
		OrderID:   "11111111-2222-4333-8444-555555555555",
		ProductID: "hh.spider.vip.monthly",
		Status:    ApplePurchaseOrderStatusCreated,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	record := &AppleTransaction{
		TransactionID:         "tx-1",
		OriginalTransactionID: "original-1",
		ProductID:             order.ProductID,
		PurchaseAt:            timePtr(now.Add(-24 * time.Hour)),
		AppAccountToken:       "",
	}

	source, err := applePurchaseOrderConfirmationSource(order, record, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("confirmation source returned error: %v", err)
	}
	if source != ApplePurchaseOrderSourcePostLoginBind {
		t.Fatalf("source = %q, want %q", source, ApplePurchaseOrderSourcePostLoginBind)
	}
}

func TestApplePurchaseOrderConfirmationSourceAllowsDelayedAnonymousBind(t *testing.T) {
	createdAt := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	order := &ApplePurchaseOrder{
		OrderID:   "11111111-2222-4333-8444-555555555555",
		ProductID: "hh.spider.vip.monthly",
		Status:    ApplePurchaseOrderStatusCreated,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(30 * time.Minute),
	}
	record := &AppleTransaction{
		TransactionID:         "tx-delayed",
		OriginalTransactionID: "original-delayed",
		ProductID:             order.ProductID,
		PurchaseAt:            timePtr(createdAt.Add(2 * time.Minute)),
		AppAccountToken:       order.OrderID,
	}

	source, err := applePurchaseOrderConfirmationSource(order, record, createdAt.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("delayed anonymous confirmation returned error: %v", err)
	}
	if source != ApplePurchaseOrderSourcePrePurchase {
		t.Fatalf("source = %q, want %q", source, ApplePurchaseOrderSourcePrePurchase)
	}
}

func TestApplePurchaseOrderConfirmationSourceExpiredOrder(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	order := &ApplePurchaseOrder{
		OrderID:   "11111111-2222-4333-8444-555555555555",
		ProductID: "hh.spider.vip.monthly",
		Status:    ApplePurchaseOrderStatusCreated,
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(-30 * time.Minute),
	}
	record := &AppleTransaction{
		TransactionID:         "tx-1",
		OriginalTransactionID: "original-1",
		ProductID:             order.ProductID,
		PurchaseAt:            timePtr(now.Add(-24 * time.Hour)),
	}

	_, err := applePurchaseOrderConfirmationSource(order, record, now)
	if !errors.Is(err, ErrApplePurchaseOrderExpired) {
		t.Fatalf("err = %v, want %v", err, ErrApplePurchaseOrderExpired)
	}
}

func TestCurrentVIPStatusFromExpiredAppleEntitlementKeepsOldFields(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Hour)
	entitlement := &UserEntitlement{
		UID:                   7,
		Entitlement:           UserEntitlementVIP,
		Kind:                  VIPKindMonthly,
		Active:                true,
		ExpiresAt:             &expiresAt,
		ProductID:             "hh.spider.vip.monthly",
		OriginalTransactionID: "original-1",
		Source:                UserEntitlementSourceApple,
	}

	status := currentVIPStatusFromEntitlement(entitlement, now)
	if status.IsVIP {
		t.Fatalf("IsVIP = true, want false")
	}
	if status.Kind != VIPKindNone {
		t.Fatalf("Kind = %q, want %q", status.Kind, VIPKindNone)
	}
	if status.ProductID != entitlement.ProductID {
		t.Fatalf("ProductID = %q, want %q", status.ProductID, entitlement.ProductID)
	}
	if status.Source != entitlement.Source {
		t.Fatalf("Source = %q, want %q", status.Source, entitlement.Source)
	}
	if status.ExpiresAt != entitlement.ExpiresAt {
		t.Fatalf("ExpiresAt = %v, want %v", status.ExpiresAt, entitlement.ExpiresAt)
	}
}

func TestCurrentVIPStatusFromEntitlementPrefersActiveAdminGrant(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	appleExpiresAt := now.Add(-time.Hour)
	adminExpiresAt := now.Add(30 * 24 * time.Hour)
	entitlement := &UserEntitlement{
		UID:                 7,
		Entitlement:         UserEntitlementVIP,
		Kind:                VIPKindMonthly,
		Active:              true,
		ExpiresAt:           &appleExpiresAt,
		ProductID:           "hh.spider.vip.monthly",
		Source:              UserEntitlementSourceApple,
		AdminGranted:        true,
		AdminGrantKind:      VIPKindMonthly,
		AdminGrantExpiresAt: &adminExpiresAt,
	}

	status := currentVIPStatusFromEntitlement(entitlement, now)
	if !status.IsVIP {
		t.Fatalf("IsVIP = false, want true")
	}
	if status.Kind != VIPKindMonthly {
		t.Fatalf("Kind = %q, want %q", status.Kind, VIPKindMonthly)
	}
	if status.Source != UserEntitlementSourceAdminGrant {
		t.Fatalf("Source = %q, want %q", status.Source, UserEntitlementSourceAdminGrant)
	}
	if status.ExpiresAt != entitlement.AdminGrantExpiresAt {
		t.Fatalf("ExpiresAt = %v, want %v", status.ExpiresAt, entitlement.AdminGrantExpiresAt)
	}
}

func TestCurrentVIPStatusFromEntitlementKeepsAppleAfterAdminGrantRevoked(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	appleExpiresAt := now.Add(30 * 24 * time.Hour)
	entitlement := &UserEntitlement{
		UID:                 7,
		Entitlement:         UserEntitlementVIP,
		Kind:                VIPKindMonthly,
		Active:              true,
		ExpiresAt:           &appleExpiresAt,
		ProductID:           "hh.spider.vip.monthly",
		Source:              UserEntitlementSourceApple,
		AdminGranted:        false,
		AdminGrantKind:      VIPKindNone,
		AdminGrantExpiresAt: nil,
	}

	status := currentVIPStatusFromEntitlement(entitlement, now)
	if !status.IsVIP {
		t.Fatalf("IsVIP = false, want true")
	}
	if status.Source != UserEntitlementSourceApple {
		t.Fatalf("Source = %q, want %q", status.Source, UserEntitlementSourceApple)
	}
	if status.ExpiresAt != entitlement.ExpiresAt {
		t.Fatalf("ExpiresAt = %v, want %v", status.ExpiresAt, entitlement.ExpiresAt)
	}
}

func TestAppleTransactionFromVerifiedPayloadKeepsOfferFields(t *testing.T) {
	transaction := appstore.Transaction{
		TransactionID:         "tx-1",
		OriginalTransactionID: "original-1",
		ProductID:             "hh.spider.vip.monthly",
		OfferIdentifier:       "SUMMER2026",
		OfferType:             3,
	}

	record := appleTransactionFromVerifiedPayload(7, transaction, "signed-jws")
	if record.OfferIdentifier != transaction.OfferIdentifier {
		t.Fatalf("OfferIdentifier = %q, want %q", record.OfferIdentifier, transaction.OfferIdentifier)
	}
	if record.OfferType != transaction.OfferType {
		t.Fatalf("OfferType = %d, want %d", record.OfferType, transaction.OfferType)
	}
}

func TestAppleTransactionFromNotificationRecordKeepsOfferFields(t *testing.T) {
	notification := &AppStoreServerNotification{
		TransactionID:         "tx-1",
		OriginalTransactionID: "original-1",
		ProductID:             "hh.spider.vip.monthly",
		OfferIdentifier:       "SUMMER2026",
		OfferType:             3,
	}

	record := appleTransactionFromNotificationRecord(7, notification)
	if record == nil {
		t.Fatal("record = nil, want AppleTransaction")
	}
	if record.OfferIdentifier != notification.OfferIdentifier {
		t.Fatalf("OfferIdentifier = %q, want %q", record.OfferIdentifier, notification.OfferIdentifier)
	}
	if record.OfferType != notification.OfferType {
		t.Fatalf("OfferType = %d, want %d", record.OfferType, notification.OfferType)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
