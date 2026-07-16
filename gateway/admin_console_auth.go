package gateway

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	appconfig "spider-server/common/config"

	"github.com/gin-gonic/gin"
)

const (
	adminTimestampHeader = "X-Admin-Timestamp"
	adminNonceHeader     = "X-Admin-Nonce"
	adminSignatureHeader = "X-Admin-Signature"
	adminMaxBodyBytes    = 1 << 20
)

type adminConsoleAuth struct {
	secret       []byte
	requireHTTPS bool
	maxClockSkew time.Duration
	mu           sync.Mutex
	nonces       map[string]time.Time
}

func newAdminConsoleAuth(cfg appconfig.AdminConfig) *adminConsoleAuth {
	return &adminConsoleAuth{
		secret:       []byte(strings.TrimSpace(cfg.ConsoleSecret)),
		requireHTTPS: cfg.ConsoleRequireHTTPS,
		maxClockSkew: cfg.ConsoleMaxClockSkewDuration(),
		nonces:       make(map[string]time.Time),
	}
}

func (a *adminConsoleAuth) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(a.secret) < 32 {
			adminError(c, http.StatusServiceUnavailable, "管理后台密钥未配置或长度不足")
			c.Abort()
			return
		}
		if a.requireHTTPS && !requestUsesHTTPS(c.Request) {
			adminError(c, http.StatusUpgradeRequired, "管理后台接口只允许通过 HTTPS 访问")
			c.Abort()
			return
		}
		if c.Request.ContentLength > adminMaxBodyBytes {
			adminError(c, http.StatusRequestEntityTooLarge, "请求体过大")
			c.Abort()
			return
		}

		body, err := io.ReadAll(io.LimitReader(c.Request.Body, adminMaxBodyBytes+1))
		if err != nil || len(body) > adminMaxBodyBytes {
			adminError(c, http.StatusBadRequest, "无法读取请求体")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		timestampText := strings.TrimSpace(c.GetHeader(adminTimestampHeader))
		nonce := strings.TrimSpace(c.GetHeader(adminNonceHeader))
		signatureText := strings.TrimSpace(c.GetHeader(adminSignatureHeader))
		if timestampText == "" || nonce == "" || signatureText == "" {
			adminError(c, http.StatusUnauthorized, "缺少管理后台签名")
			c.Abort()
			return
		}
		if len(nonce) < 16 || len(nonce) > 128 || strings.ContainsAny(nonce, "\r\n\t ") {
			adminError(c, http.StatusUnauthorized, "管理后台 nonce 无效")
			c.Abort()
			return
		}
		timestamp, err := strconv.ParseInt(timestampText, 10, 64)
		if err != nil {
			adminError(c, http.StatusUnauthorized, "管理后台时间戳无效")
			c.Abort()
			return
		}
		now := time.Now()
		requestTime := time.Unix(timestamp, 0)
		if requestTime.Before(now.Add(-a.maxClockSkew)) || requestTime.After(now.Add(a.maxClockSkew)) {
			adminError(c, http.StatusUnauthorized, "管理后台请求已过期")
			c.Abort()
			return
		}

		provided, err := hex.DecodeString(signatureText)
		if err != nil || len(provided) != sha256.Size {
			adminError(c, http.StatusUnauthorized, "管理后台签名无效")
			c.Abort()
			return
		}
		expected := adminRequestSignature(a.secret, c.Request.Method, c.Request.URL.RequestURI(), timestampText, nonce, body)
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			adminError(c, http.StatusUnauthorized, "管理后台签名无效")
			c.Abort()
			return
		}
		if !a.useNonce(nonce, now) {
			adminError(c, http.StatusConflict, "检测到重复的管理后台请求")
			c.Abort()
			return
		}

		c.Next()
	}
}

func adminRequestSignature(secret []byte, method string, requestURI string, timestamp string, nonce string, body []byte) []byte {
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		requestURI,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return mac.Sum(nil)
}

func (a *adminConsoleAuth) useNonce(nonce string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, expiresAt := range a.nonces {
		if !expiresAt.After(now) {
			delete(a.nonces, key)
		}
	}
	if expiresAt, exists := a.nonces[nonce]; exists && expiresAt.After(now) {
		return false
	}
	a.nonces[nonce] = now.Add(2 * a.maxClockSkew)
	return true
}

func requestUsesHTTPS(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil {
		return false
	}
	proto := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	return proto == "https"
}
