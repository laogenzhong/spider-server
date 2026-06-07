package reconcile

import (
	"context"
	"fmt"
	"log"
	appconfig "spider-server/common/config"
	"spider-server/game/appstore"
	mysqlmodel "spider-server/mysql/model"
	"strings"
	"time"
)

func StartAppStoreReconciler(ctx context.Context, cfg appconfig.AppStoreConfig) {
	if !cfg.ReconcileEnabled {
		return
	}

	api := appstore.DefaultServerAPI()
	if !api.Configured() {
		log.Println("app store reconcile disabled: server api config is incomplete")
		return
	}

	interval := cfg.ReconcileIntervalDuration()
	log.Printf("app store reconcile started: interval=%s lookback=%s batch=%d max_pages=%d", interval, cfg.ReconcileLookbackDuration(), cfg.ReconcileBatchSize, cfg.ReconcileMaxPages)

	go func() {
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("app store reconcile stopped")
				return
			case <-timer.C:
				if err := RunAppStoreReconcileOnce(ctx, cfg); err != nil {
					log.Printf("app store reconcile completed with errors: %v", err)
				}
				timer.Reset(interval)
			}
		}
	}()
}

func RunAppStoreReconcileOnce(ctx context.Context, cfg appconfig.AppStoreConfig) error {
	api := appstore.DefaultServerAPI()
	if !api.Configured() {
		return appstore.ErrServerAPIConfigInvalid
	}

	now := time.Now()
	startAt := now.Add(-cfg.ReconcileLookbackDuration())
	failures := 0

	onlyFailures := true
	if history, err := api.GetNotificationHistory(ctx, "", startAt, now, &onlyFailures, cfg.ReconcileMaxPages); err != nil {
		failures++
		log.Printf("app store reconcile notification history failed: %v", err)
		recordAppStoreReconcileFailure(
			mysqlmodel.ApplePaymentFailureStageReconcile,
			"App Store notification history query failed, so missed notifications may remain unapplied.",
			err,
			map[string]any{"scope": "failed_notifications", "startAt": startAt, "endAt": now},
		)
	} else if count, err := applyNotificationHistory(history, cfg, now); err != nil {
		failures++
		log.Printf("app store reconcile apply notification history failed: count=%d err=%v", count, err)
	}

	refs, err := mysqlmodel.ListAppleTransactionsForAppStoreReconcile(cfg.ReconcileBatchSize)
	if err != nil {
		return err
	}

	for _, ref := range refs {
		transactionID := firstNonEmpty(ref.OriginalTransactionID, ref.TransactionID)
		if transactionID == "" {
			continue
		}

		if history, err := api.GetTransactionHistory(ctx, transactionID, startAt, now, cfg.ReconcileMaxPages); err != nil {
			failures++
			log.Printf("app store reconcile transaction history failed: uid=%d tx=%s err=%v", ref.UID, transactionID, err)
			recordAppStoreReconcileFailure(
				mysqlmodel.ApplePaymentFailureStageReconcile,
				"App Store transaction history query failed, so local transaction and entitlement snapshots may be stale.",
				err,
				map[string]any{"uid": ref.UID, "transactionID": transactionID, "startAt": startAt, "endAt": now},
			)
		} else if err := mysqlmodel.ApplyAppStoreServerAPITransactions(ref.UID, history.Transactions, cfg.MonthlyProductID, cfg.LifetimeProductID, now); err != nil {
			failures++
			log.Printf("app store reconcile apply transaction history failed: uid=%d tx=%s err=%v", ref.UID, transactionID, err)
			if transactionHistoryHasRevocation(history.Transactions) {
				recordRefundRevokeReconcileFailure(
					ref.UID,
					transactionID,
					"Applying App Store transaction history with revoked transactions failed, so a VIP entitlement may not have been revoked correctly.",
					err,
					map[string]any{"uid": ref.UID, "transactionID": transactionID, "transactionCount": len(history.Transactions)},
				)
			} else {
				recordAppStoreReconcileFailure(
					mysqlmodel.ApplePaymentFailureStageReconcile,
					"Applying App Store transaction history failed, so local VIP entitlement may be stale.",
					err,
					map[string]any{"uid": ref.UID, "transactionID": transactionID, "transactionCount": len(history.Transactions)},
				)
			}
		}

		if strings.TrimSpace(ref.ProductID) == strings.TrimSpace(cfg.MonthlyProductID) {
			if status, err := api.GetSubscriptionStatus(ctx, transactionID); err != nil {
				failures++
				log.Printf("app store reconcile subscription status failed: uid=%d tx=%s err=%v", ref.UID, transactionID, err)
				recordAppStoreReconcileFailure(
					mysqlmodel.ApplePaymentFailureStageReconcile,
					"App Store subscription status query failed, so monthly VIP status may be stale.",
					err,
					map[string]any{"uid": ref.UID, "transactionID": transactionID},
				)
			} else if err := mysqlmodel.ApplyAppStoreSubscriptionStatuses(ref.UID, status.Items, cfg.MonthlyProductID, cfg.LifetimeProductID, now); err != nil {
				failures++
				log.Printf("app store reconcile apply subscription status failed: uid=%d tx=%s err=%v", ref.UID, transactionID, err)
				if subscriptionStatusHasRevocation(status.Items) {
					recordRefundRevokeReconcileFailure(
						ref.UID,
						transactionID,
						"Applying App Store revoked subscription status failed, so a VIP entitlement may not have been revoked correctly.",
						err,
						map[string]any{"uid": ref.UID, "transactionID": transactionID, "statusCount": len(status.Items)},
					)
				} else {
					recordAppStoreReconcileFailure(
						mysqlmodel.ApplePaymentFailureStageReconcile,
						"Applying App Store subscription status failed, so monthly VIP entitlement may be stale.",
						err,
						map[string]any{"uid": ref.UID, "transactionID": transactionID, "statusCount": len(status.Items)},
					)
				}
			}
		}

		if history, err := api.GetNotificationHistory(ctx, transactionID, startAt, now, nil, cfg.ReconcileMaxPages); err != nil {
			failures++
			log.Printf("app store reconcile transaction notification history failed: uid=%d tx=%s err=%v", ref.UID, transactionID, err)
			recordAppStoreReconcileFailure(
				mysqlmodel.ApplePaymentFailureStageReconcile,
				"App Store transaction notification history query failed, so missed notifications may remain unapplied.",
				err,
				map[string]any{"uid": ref.UID, "transactionID": transactionID, "startAt": startAt, "endAt": now},
			)
		} else if count, err := applyNotificationHistory(history, cfg, now); err != nil {
			failures++
			log.Printf("app store reconcile apply transaction notification history failed: uid=%d tx=%s count=%d err=%v", ref.UID, transactionID, count, err)
		}
	}

	if err := mysqlmodel.RecordPendingAppStoreNotificationBacklog(now); err != nil {
		failures++
		log.Printf("app store reconcile pending_user backlog check failed: %v", err)
	}

	if failures > 0 {
		return fmt.Errorf("%d app store reconcile step(s) failed", failures)
	}
	log.Printf("app store reconcile completed: refs=%d", len(refs))
	return nil
}

func applyNotificationHistory(history appstore.NotificationHistoryResponse, cfg appconfig.AppStoreConfig, now time.Time) (int, error) {
	count := 0
	for _, item := range history.Notifications {
		if strings.TrimSpace(item.Notification.NotificationUUID) == "" {
			continue
		}
		if err := mysqlmodel.SaveAppStoreServerNotificationAndApplyVIP(
			item.Notification,
			item.Transaction,
			item.RenewalInfo,
			item.SignedPayload,
			cfg.MonthlyProductID,
			cfg.LifetimeProductID,
			now,
		); err != nil {
			recordNotificationHistoryApplyFailure(item, err, now)
			return count, err
		}
		count++
	}
	return count, nil
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

func recordNotificationHistoryApplyFailure(item appstore.NotificationHistoryItem, err error, now time.Time) {
	category := mysqlmodel.ApplePaymentFailureCategoryNotification5xx
	stage := mysqlmodel.ApplePaymentFailureStageNotificationApply
	problem := "Applying App Store notification history failed, so a missed notification may still be unapplied."
	if isRefundOrRevokeNotification(item.Notification, item.Transaction) {
		category = mysqlmodel.ApplePaymentFailureCategoryRefundRevoke
		stage = mysqlmodel.ApplePaymentFailureStageRefundRevokeApply
		problem = "Applying refund or revoke notification from history failed, so a VIP entitlement may not have been revoked correctly."
	}

	transactionID := ""
	originalTransactionID := ""
	productID := ""
	environment := item.Notification.Data.Environment
	if item.Transaction != nil {
		transactionID = item.Transaction.TransactionID
		originalTransactionID = item.Transaction.OriginalTransactionID
		productID = item.Transaction.ProductID
		environment = firstNonEmpty(environment, item.Transaction.Environment)
	}
	if item.RenewalInfo != nil {
		originalTransactionID = firstNonEmpty(originalTransactionID, item.RenewalInfo.OriginalTransactionID)
		productID = firstNonEmpty(productID, item.RenewalInfo.ProductID)
		environment = firstNonEmpty(environment, item.RenewalInfo.Environment)
	}

	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:              category,
		Stage:                 stage,
		Severity:              mysqlmodel.ApplePaymentFailureSeverityCritical,
		ProductID:             productID,
		TransactionID:         transactionID,
		OriginalTransactionID: originalTransactionID,
		NotificationUUID:      item.Notification.NotificationUUID,
		NotificationType:      item.Notification.NotificationType,
		Subtype:               item.Notification.Subtype,
		BundleID:              item.Notification.Data.BundleID,
		Environment:           environment,
		Reason:                errString(err),
		Problem:               problem,
		ErrorMessage:          errString(err),
		ContextJSON: mysqlmodel.ApplePaymentFailureContext(map[string]any{
			"sendAttempts":        item.SendAttempts,
			"signedPayloadLength": len(strings.TrimSpace(item.SignedPayload)),
			"signedPayloadDots":   strings.Count(strings.TrimSpace(item.SignedPayload), "."),
		}),
		OccurredAt: now,
	})
}

func recordAppStoreReconcileFailure(stage string, problem string, err error, context map[string]any) {
	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:     mysqlmodel.ApplePaymentFailureCategoryReconcile,
		Stage:        stage,
		Severity:     mysqlmodel.ApplePaymentFailureSeverityWarning,
		Reason:       errString(err),
		Problem:      problem,
		ErrorMessage: errString(err),
		ContextJSON:  mysqlmodel.ApplePaymentFailureContext(context),
		OccurredAt:   time.Now(),
	})
}

func recordRefundRevokeReconcileFailure(uid uint64, transactionID string, problem string, err error, context map[string]any) {
	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:      mysqlmodel.ApplePaymentFailureCategoryRefundRevoke,
		Stage:         mysqlmodel.ApplePaymentFailureStageRefundRevokeApply,
		Severity:      mysqlmodel.ApplePaymentFailureSeverityCritical,
		UID:           uid,
		TransactionID: transactionID,
		Reason:        errString(err),
		Problem:       problem,
		ErrorMessage:  errString(err),
		ContextJSON:   mysqlmodel.ApplePaymentFailureContext(context),
		OccurredAt:    time.Now(),
	})
}

func isRefundOrRevokeNotification(notification appstore.Notification, transaction *appstore.Transaction) bool {
	notificationType := strings.ToUpper(strings.TrimSpace(notification.NotificationType))
	if notificationType == "REFUND" || notificationType == "REVOKE" {
		return true
	}
	return transaction != nil && transaction.RevocationDate > 0
}

func transactionHistoryHasRevocation(transactions []appstore.VerifiedTransaction) bool {
	for _, transaction := range transactions {
		if transaction.Transaction.RevocationDate > 0 {
			return true
		}
	}
	return false
}

func subscriptionStatusHasRevocation(items []appstore.SubscriptionStatusItem) bool {
	for _, item := range items {
		if item.Status == mysqlmodel.AppStoreSubscriptionStatusRevoked {
			return true
		}
		if item.Transaction != nil && item.Transaction.RevocationDate > 0 {
			return true
		}
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
