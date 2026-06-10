package mysqlmodel

import (
	"errors"
	"testing"
	"time"
)

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

func timePtr(t time.Time) *time.Time {
	return &t
}
