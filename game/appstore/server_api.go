package appstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	appconfig "spider-server/common/config"
	"strings"
	"time"
)

var (
	ErrServerAPIConfigInvalid = errors.New("app store server api config invalid")
	ErrServerAPIRequest       = errors.New("app store server api request failed")

	defaultServerAPI = NewServerAPI(appconfig.Default().AppStore)
)

type ServerAPI struct {
	cfg appconfig.AppStoreConfig
}

type serverAPIRequest struct {
	Action               string   `json:"action"`
	BundleID             string   `json:"bundleId"`
	Environment          string   `json:"environment"`
	AppAppleID           int64    `json:"appAppleId"`
	EnableOnlineChecks   bool     `json:"enableOnlineChecks"`
	RootCertificatePaths []string `json:"rootCertificatePaths"`
	APIKeyID             string   `json:"apiKeyId"`
	APIIssuerID          string   `json:"apiIssuerId"`
	APIPrivateKeyPath    string   `json:"apiPrivateKeyPath"`
	APIPrivateKey        string   `json:"apiPrivateKey"`
	TransactionID        string   `json:"transactionId,omitempty"`
	Revision             string   `json:"revision,omitempty"`
	PaginationToken      string   `json:"paginationToken,omitempty"`
	StartDate            int64    `json:"startDate,omitempty"`
	EndDate              int64    `json:"endDate,omitempty"`
	ProductIDs           []string `json:"productIds,omitempty"`
	OnlyFailures         *bool    `json:"onlyFailures,omitempty"`
	NotificationType     string   `json:"notificationType,omitempty"`
	NotificationSubtype  string   `json:"notificationSubtype,omitempty"`
	MaxPages             int      `json:"maxPages,omitempty"`
}

type serverAPIEnvelope struct {
	OK             bool            `json:"ok"`
	Action         string          `json:"action"`
	Error          string          `json:"error"`
	HTTPStatusCode int             `json:"httpStatusCode"`
	APIError       any             `json:"apiError"`
	Data           json.RawMessage `json:"data"`
}

type TransactionHistoryResponse struct {
	Pages        []TransactionHistoryPage `json:"pages"`
	Transactions []VerifiedTransaction    `json:"transactions"`
	Revision     string                   `json:"revision"`
	HasMore      bool                     `json:"hasMore"`
}

type TransactionHistoryPage struct {
	Revision    string `json:"revision"`
	HasMore     bool   `json:"hasMore"`
	BundleID    string `json:"bundleId"`
	AppAppleID  int64  `json:"appAppleId"`
	Environment string `json:"environment"`
	Count       int    `json:"count"`
}

type VerifiedTransaction struct {
	SignedTransactionJWS string      `json:"signedTransactionJWS"`
	Transaction          Transaction `json:"transaction"`
}

type SubscriptionStatusResponse struct {
	Environment string                   `json:"environment"`
	BundleID    string                   `json:"bundleId"`
	AppAppleID  int64                    `json:"appAppleId"`
	Items       []SubscriptionStatusItem `json:"items"`
}

type SubscriptionStatusItem struct {
	SubscriptionGroupIdentifier string       `json:"subscriptionGroupIdentifier"`
	Status                      int32        `json:"status"`
	OriginalTransactionID       string       `json:"originalTransactionId"`
	SignedTransactionJWS        string       `json:"signedTransactionJWS"`
	SignedRenewalInfoJWS        string       `json:"signedRenewalInfoJWS"`
	Transaction                 *Transaction `json:"transaction"`
	RenewalInfo                 *RenewalInfo `json:"renewalInfo"`
}

type NotificationHistoryResponse struct {
	Pages           []NotificationHistoryPage `json:"pages"`
	Notifications   []NotificationHistoryItem `json:"notifications"`
	PaginationToken string                    `json:"paginationToken"`
	HasMore         bool                      `json:"hasMore"`
}

type NotificationHistoryPage struct {
	PaginationToken string `json:"paginationToken"`
	HasMore         bool   `json:"hasMore"`
	Count           int    `json:"count"`
}

type NotificationHistoryItem struct {
	SignedPayload string           `json:"signedPayload"`
	SendAttempts  []map[string]any `json:"sendAttempts"`
	Notification  Notification     `json:"notification"`
	Transaction   *Transaction     `json:"transaction"`
	RenewalInfo   *RenewalInfo     `json:"renewalInfo"`
}

func ConfigureServerAPI(cfg appconfig.AppStoreConfig) {
	defaultServerAPI = NewServerAPI(cfg)
}

func DefaultServerAPI() *ServerAPI {
	return defaultServerAPI
}

func NewServerAPI(cfg appconfig.AppStoreConfig) *ServerAPI {
	return &ServerAPI{cfg: cfg}
}

func (c *ServerAPI) Config() appconfig.AppStoreConfig {
	return c.cfg
}

func (c *ServerAPI) Configured() bool {
	return c.validateConfig() == nil
}

func (c *ServerAPI) GetTransactionHistory(ctx context.Context, transactionID string, startAt time.Time, endAt time.Time, maxPages int) (TransactionHistoryResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return TransactionHistoryResponse{}, fmt.Errorf("%w: transactionId is empty", ErrServerAPIRequest)
	}

	request := c.baseRequest("transactionHistory")
	request.TransactionID = transactionID
	request.ProductIDs = configuredProductIDs(c.cfg)
	request.MaxPages = maxPages
	if !startAt.IsZero() {
		request.StartDate = startAt.UnixMilli()
	}
	if !endAt.IsZero() {
		request.EndDate = endAt.UnixMilli()
	}

	var response TransactionHistoryResponse
	if err := c.runScript(ctx, request, &response); err != nil {
		return TransactionHistoryResponse{}, err
	}
	return response, nil
}

func (c *ServerAPI) GetSubscriptionStatus(ctx context.Context, transactionID string) (SubscriptionStatusResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return SubscriptionStatusResponse{}, fmt.Errorf("%w: transactionId is empty", ErrServerAPIRequest)
	}

	request := c.baseRequest("subscriptionStatus")
	request.TransactionID = transactionID

	var response SubscriptionStatusResponse
	if err := c.runScript(ctx, request, &response); err != nil {
		return SubscriptionStatusResponse{}, err
	}
	return response, nil
}

func (c *ServerAPI) GetNotificationHistory(ctx context.Context, transactionID string, startAt time.Time, endAt time.Time, onlyFailures *bool, maxPages int) (NotificationHistoryResponse, error) {
	request := c.baseRequest("notificationHistory")
	request.TransactionID = strings.TrimSpace(transactionID)
	request.OnlyFailures = onlyFailures
	request.MaxPages = maxPages
	if !startAt.IsZero() {
		request.StartDate = startAt.UnixMilli()
	}
	if !endAt.IsZero() {
		request.EndDate = endAt.UnixMilli()
	}

	var response NotificationHistoryResponse
	if err := c.runScript(ctx, request, &response); err != nil {
		return NotificationHistoryResponse{}, err
	}
	return response, nil
}

func (c *ServerAPI) baseRequest(action string) serverAPIRequest {
	return serverAPIRequest{
		Action:               action,
		BundleID:             c.cfg.BundleID,
		Environment:          c.cfg.Environment,
		AppAppleID:           c.cfg.AppAppleID,
		EnableOnlineChecks:   c.cfg.EnableOnlineChecks,
		RootCertificatePaths: c.cfg.RootCertificatePaths,
		APIKeyID:             c.cfg.APIKeyID,
		APIIssuerID:          c.cfg.APIIssuerID,
		APIPrivateKeyPath:    c.cfg.APIPrivateKeyPath,
		APIPrivateKey:        c.cfg.APIPrivateKey,
	}
}

func (c *ServerAPI) runScript(ctx context.Context, request serverAPIRequest, data any) error {
	if err := c.validateConfig(); err != nil {
		return err
	}

	timeout := c.cfg.TimeoutDuration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("%w: marshal request: %v", ErrServerAPIRequest, err)
	}

	cmd := exec.CommandContext(ctx, c.cfg.NodePath, c.cfg.APIScriptPath)
	cmd.Stdin = bytes.NewReader(requestBytes)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stdout.String())
		if message == "" {
			message = strings.TrimSpace(stderr.String())
		}
		return fmt.Errorf("%w: %s", ErrServerAPIRequest, message)
	}

	var envelope serverAPIEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrServerAPIRequest, err)
	}
	if !envelope.OK {
		return fmt.Errorf("%w: http=%d api=%v %s", ErrServerAPIRequest, envelope.HTTPStatusCode, envelope.APIError, envelope.Error)
	}
	if err := json.Unmarshal(envelope.Data, data); err != nil {
		return fmt.Errorf("%w: decode data: %v", ErrServerAPIRequest, err)
	}

	return nil
}

func (c *ServerAPI) validateConfig() error {
	if strings.TrimSpace(c.cfg.NodePath) == "" ||
		strings.TrimSpace(c.cfg.APIScriptPath) == "" ||
		strings.TrimSpace(c.cfg.BundleID) == "" ||
		strings.TrimSpace(c.cfg.Environment) == "" ||
		strings.TrimSpace(c.cfg.APIKeyID) == "" ||
		strings.TrimSpace(c.cfg.APIIssuerID) == "" ||
		(strings.TrimSpace(c.cfg.APIPrivateKeyPath) == "" && strings.TrimSpace(c.cfg.APIPrivateKey) == "") ||
		len(c.cfg.RootCertificatePaths) == 0 {
		return ErrServerAPIConfigInvalid
	}
	return nil
}

func configuredProductIDs(cfg appconfig.AppStoreConfig) []string {
	ids := make([]string, 0, 2)
	if value := strings.TrimSpace(cfg.MonthlyProductID); value != "" {
		ids = append(ids, value)
	}
	if value := strings.TrimSpace(cfg.LifetimeProductID); value != "" {
		ids = append(ids, value)
	}
	return ids
}
