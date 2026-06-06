package gateway

import (
	"errors"
	"log"
	"net/http"
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
		log.Printf("app store notification verifier config invalid: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code": http.StatusServiceUnavailable,
			"msg":  "app store verifier is not configured",
		})
		return
	}
	if err != nil {
		log.Printf("app store notification verify failed: %v", err)
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
		log.Printf("app store notification save/apply failed: uuid=%s type=%s subtype=%s err=%v",
			notification.NotificationUUID,
			notification.NotificationType,
			notification.Subtype,
			err,
		)
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
