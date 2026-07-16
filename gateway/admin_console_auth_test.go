package gateway

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	appconfig "spider-server/common/config"

	"github.com/gin-gonic/gin"
)

func TestAdminRequestSignatureChangesWithBody(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	first := adminRequestSignature(secret, "POST", "/admin-console/vip/grant", "100", "0123456789abcdef", []byte(`{"days":7}`))
	second := adminRequestSignature(secret, "POST", "/admin-console/vip/grant", "100", "0123456789abcdef", []byte(`{"days":30}`))
	if hex.EncodeToString(first) == hex.EncodeToString(second) {
		t.Fatal("signature should change when request body changes")
	}
}

func TestAdminConsoleMiddlewareAcceptsSignedRequestAndRejectsReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "0123456789abcdef0123456789abcdef"
	auth := newAdminConsoleAuth(appconfig.AdminConfig{
		ConsoleSecret:       secret,
		ConsoleMaxClockSkew: "90s",
	})
	router := gin.New()
	router.GET("/admin-console/health", auth.middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "0123456789abcdef0123456789abcdef"
	signature := hex.EncodeToString(adminRequestSignature(
		[]byte(secret),
		http.MethodGet,
		"/admin-console/health?check=1",
		timestamp,
		nonce,
		nil,
	))
	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/admin-console/health?check=1", nil)
		req.Header.Set(adminTimestampHeader, timestamp)
		req.Header.Set(adminNonceHeader, nonce)
		req.Header.Set(adminSignatureHeader, signature)
		return req
	}

	first := httptest.NewRecorder()
	router.ServeHTTP(first, newRequest())
	if first.Code != http.StatusOK {
		t.Fatalf("first signed request status = %d, want %d; body=%s", first.Code, http.StatusOK, first.Body.String())
	}
	second := httptest.NewRecorder()
	router.ServeHTTP(second, newRequest())
	if second.Code != http.StatusConflict {
		t.Fatalf("replayed request status = %d, want %d; body=%s", second.Code, http.StatusConflict, second.Body.String())
	}
}

func TestAdminConsoleMiddlewareRejectsExpiredAndInvalidSignatures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "0123456789abcdef0123456789abcdef"
	auth := newAdminConsoleAuth(appconfig.AdminConfig{
		ConsoleSecret:       secret,
		ConsoleMaxClockSkew: "1s",
	})
	router := gin.New()
	router.POST("/admin-console/vip/grant", auth.middleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	expired := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/admin-console/vip/grant", nil)
	req.Header.Set(adminTimestampHeader, expired)
	req.Header.Set(adminNonceHeader, "0123456789abcdef")
	req.Header.Set(adminSignatureHeader, strings.Repeat("0", 64))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expired request status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req = httptest.NewRequest(http.MethodPost, "/admin-console/vip/grant", nil)
	req.Header.Set(adminTimestampHeader, timestamp)
	req.Header.Set(adminNonceHeader, "fedcba9876543210")
	req.Header.Set(adminSignatureHeader, strings.Repeat("0", 64))
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAdminConsoleMiddlewareRequiresHTTPSWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := newAdminConsoleAuth(appconfig.AdminConfig{
		ConsoleSecret:       "0123456789abcdef0123456789abcdef",
		ConsoleRequireHTTPS: true,
		ConsoleMaxClockSkew: "90s",
	})
	router := gin.New()
	router.GET("/admin-console/health", auth.middleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/admin-console/health", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUpgradeRequired {
		t.Fatalf("insecure request status = %d, want %d", recorder.Code, http.StatusUpgradeRequired)
	}
}

func TestAdminConsoleNonceRejectsReplay(t *testing.T) {
	auth := newAdminConsoleAuth(appconfig.AdminConfig{
		ConsoleSecret:       "0123456789abcdef0123456789abcdef",
		ConsoleMaxClockSkew: "90s",
	})
	now := time.Now()
	if !auth.useNonce("0123456789abcdef", now) {
		t.Fatal("first nonce use should succeed")
	}
	if auth.useNonce("0123456789abcdef", now.Add(time.Second)) {
		t.Fatal("replayed nonce should fail")
	}
	if !auth.useNonce("fedcba9876543210", now.Add(time.Second)) {
		t.Fatal("different nonce should succeed")
	}
}

func TestAdminConsoleNonceExpires(t *testing.T) {
	auth := newAdminConsoleAuth(appconfig.AdminConfig{
		ConsoleSecret:       "0123456789abcdef0123456789abcdef",
		ConsoleMaxClockSkew: "1s",
	})
	now := time.Now()
	if !auth.useNonce("0123456789abcdef", now) {
		t.Fatal("first nonce use should succeed")
	}
	if !auth.useNonce("0123456789abcdef", now.Add(3*time.Second)) {
		t.Fatal("expired nonce should be reusable")
	}
}
