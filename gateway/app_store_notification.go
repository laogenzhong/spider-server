package gateway

import (
	"errors"
	"net/http"
	applogger "spider-server/common/logger"
	"strings"
	"time"

	"spider-server/game/appstore"
	mysqlmodel "spider-server/mysql/model"

	"github.com/gin-gonic/gin"
)

type appStoreServerNotificationV2Request struct {
	SignedPayload string `json:"signedPayload"`
}

func (s *GatewayServer) appStoreServerNotificationV2Handler(c *gin.Context) {
	var req appStoreServerNotificationV2Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request body",
		})
		return
	}

	signedPayload := strings.TrimSpace(req.SignedPayload)
	if signedPayload == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "signedPayload is empty",
		})
		return
	}

	verifier := appstore.DefaultVerifier()
	notification, transaction, renewalInfo, err := verifier.VerifyNotification(c.Request.Context(), signedPayload)
	if errors.Is(err, appstore.ErrVerifierConfigInvalid) {
		applogger.Printf("app store notification verifier config invalid: %v", err)
		recordAppStoreNotificationVerifyFailure(
			signedPayload,
			http.StatusServiceUnavailable,
			mysqlmodel.ApplePaymentFailureCategoryNotification5xx,
			mysqlmodel.ApplePaymentFailureSeverityCritical,
			"App Store notification verifier config is invalid, so Apple receives 503 and may retry.",
			err,
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code": http.StatusServiceUnavailable,
			"msg":  "app store verifier is not configured",
		})
		return
	}
	if err != nil {
		applogger.Printf("app store notification verify failed: %v", err)
		recordAppStoreNotificationVerifyFailure(
			signedPayload,
			http.StatusBadRequest,
			mysqlmodel.ApplePaymentFailureCategoryNotificationVerify,
			mysqlmodel.ApplePaymentFailureSeverityWarning,
			"App Store notification signedPayload verification failed, so the notification is not trusted or applied.",
			err,
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "app store notification verify failed",
		})
		return
	}

	cfg := verifier.Config()
	if err := mysqlmodel.SaveAppStoreServerNotificationAndApplyVIP(
		notification,
		transaction,
		renewalInfo,
		signedPayload,
		cfg.MonthlyProductID,
		cfg.LifetimeProductID,
		time.Now(),
	); err != nil {
		applogger.Printf("app store notification save/apply failed: uuid=%s type=%s subtype=%s err=%v",
			notification.NotificationUUID,
			notification.NotificationType,
			notification.Subtype,
			err,
		)
		recordAppStoreNotificationProcessingFailure(notification, transaction, renewalInfo, signedPayload, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "app store notification processing failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
	})
}

func recordAppStoreNotificationVerifyFailure(
	signedPayload string,
	httpStatus int,
	category string,
	severity string,
	problem string,
	err error,
) {
	signedPayload = strings.TrimSpace(signedPayload)
	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:     category,
		Stage:        mysqlmodel.ApplePaymentFailureStageNotificationVerify,
		Severity:     severity,
		HTTPStatus:   httpStatus,
		Reason:       errString(err),
		Problem:      problem,
		ErrorMessage: errString(err),
		ContextJSON: mysqlmodel.ApplePaymentFailureContext(map[string]any{
			"signedPayloadLength": len(signedPayload),
			"signedPayloadDots":   strings.Count(signedPayload, "."),
		}),
		OccurredAt: time.Now(),
	})
}

func recordAppStoreNotificationProcessingFailure(
	notification appstore.Notification,
	transaction *appstore.Transaction,
	renewalInfo *appstore.RenewalInfo,
	signedPayload string,
	err error,
) {
	category := mysqlmodel.ApplePaymentFailureCategoryNotification5xx
	stage := mysqlmodel.ApplePaymentFailureStageNotificationApply
	severity := mysqlmodel.ApplePaymentFailureSeverityCritical
	problem := "App Store notification processing returned 500, so Apple may retry and the entitlement change was not fully applied."
	if isRefundOrRevokeNotification(notification, transaction) {
		category = mysqlmodel.ApplePaymentFailureCategoryRefundRevoke
		stage = mysqlmodel.ApplePaymentFailureStageRefundRevokeApply
		problem = "Refund or revoke notification processing failed, so a VIP entitlement may not have been revoked correctly."
	}

	transactionID := ""
	originalTransactionID := ""
	productID := ""
	if transaction != nil {
		transactionID = transaction.TransactionID
		originalTransactionID = transaction.OriginalTransactionID
		productID = transaction.ProductID
	}
	if renewalInfo != nil {
		originalTransactionID = firstNonEmpty(originalTransactionID, renewalInfo.OriginalTransactionID)
		productID = firstNonEmpty(productID, renewalInfo.ProductID)
	}

	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:              category,
		Stage:                 stage,
		Severity:              severity,
		ProductID:             productID,
		TransactionID:         transactionID,
		OriginalTransactionID: originalTransactionID,
		NotificationUUID:      notification.NotificationUUID,
		NotificationType:      notification.NotificationType,
		Subtype:               notification.Subtype,
		BundleID:              notification.Data.BundleID,
		Environment:           firstNonEmpty(notification.Data.Environment, transactionEnvironment(transaction), renewalEnvironment(renewalInfo)),
		HTTPStatus:            http.StatusInternalServerError,
		Reason:                errString(err),
		Problem:               problem,
		ErrorMessage:          errString(err),
		ContextJSON: mysqlmodel.ApplePaymentFailureContext(map[string]any{
			"appAppleID":             notification.Data.AppAppleID,
			"bundleVersion":          notification.Data.BundleVersion,
			"subscriptionStatus":     notification.Data.Status,
			"consumptionReason":      notification.Data.ConsumptionReason,
			"autoRenewProductID":     renewalAutoRenewProductID(renewalInfo),
			"autoRenewStatus":        renewalAutoRenewStatus(renewalInfo),
			"expirationIntent":       renewalExpirationIntent(renewalInfo),
			"isInBillingRetryPeriod": renewalIsInBillingRetryPeriod(renewalInfo),
			"signedPayloadLength":    len(strings.TrimSpace(signedPayload)),
			"signedPayloadDots":      strings.Count(strings.TrimSpace(signedPayload), "."),
		}),
		OccurredAt: time.Now(),
	})
}

func isRefundOrRevokeNotification(notification appstore.Notification, transaction *appstore.Transaction) bool {
	notificationType := strings.ToUpper(strings.TrimSpace(notification.NotificationType))
	if notificationType == "REFUND" || notificationType == "REVOKE" {
		return true
	}
	return transaction != nil && transaction.RevocationDate > 0
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

func transactionEnvironment(transaction *appstore.Transaction) string {
	if transaction == nil {
		return ""
	}
	return transaction.Environment
}

func renewalEnvironment(renewalInfo *appstore.RenewalInfo) string {
	if renewalInfo == nil {
		return ""
	}
	return renewalInfo.Environment
}

func renewalAutoRenewProductID(renewalInfo *appstore.RenewalInfo) string {
	if renewalInfo == nil {
		return ""
	}
	return renewalInfo.AutoRenewProductID
}

func renewalAutoRenewStatus(renewalInfo *appstore.RenewalInfo) int32 {
	if renewalInfo == nil {
		return 0
	}
	return renewalInfo.AutoRenewStatus
}

func renewalExpirationIntent(renewalInfo *appstore.RenewalInfo) int32 {
	if renewalInfo == nil {
		return 0
	}
	return renewalInfo.ExpirationIntent
}

func renewalIsInBillingRetryPeriod(renewalInfo *appstore.RenewalInfo) bool {
	return renewalInfo != nil && renewalInfo.IsInBillingRetryPeriod
}
