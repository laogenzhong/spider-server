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
	ErrVerifierConfigInvalid = errors.New("app store verifier config invalid")
	ErrVerifyTransaction     = errors.New("app store transaction verify failed")

	defaultVerifier = NewVerifier(appconfig.Default().AppStore)
)

type Verifier struct {
	cfg appconfig.AppStoreConfig
}

type VerifyTransactionRequest struct {
	SignedTransactionJWS string   `json:"signedTransactionJWS"`
	BundleID             string   `json:"bundleId"`
	Environment          string   `json:"environment"`
	AppAppleID           int64    `json:"appAppleId"`
	EnableOnlineChecks   bool     `json:"enableOnlineChecks"`
	RootCertificatePaths []string `json:"rootCertificatePaths"`
}

type VerifyTransactionResponse struct {
	OK          bool        `json:"ok"`
	Error       string      `json:"error"`
	Transaction Transaction `json:"transaction"`
}

type Transaction struct {
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	BundleID              string `json:"bundleId"`
	ProductID             string `json:"productId"`
	Environment           string `json:"environment"`
	Type                  string `json:"type"`
	PurchaseDate          int64  `json:"purchaseDate"`
	OriginalPurchaseDate  int64  `json:"originalPurchaseDate"`
	ExpiresDate           int64  `json:"expiresDate"`
	RevocationDate        int64  `json:"revocationDate"`
	RevocationReason      int32  `json:"revocationReason"`
	SignedDate            int64  `json:"signedDate"`
}

func Configure(cfg appconfig.AppStoreConfig) {
	defaultVerifier = NewVerifier(cfg)
}

func DefaultVerifier() *Verifier {
	return defaultVerifier
}

func NewVerifier(cfg appconfig.AppStoreConfig) *Verifier {
	return &Verifier{cfg: cfg}
}

func (v *Verifier) Config() appconfig.AppStoreConfig {
	return v.cfg
}

func (v *Verifier) VerifyTransaction(ctx context.Context, signedTransactionJWS string) (Transaction, error) {
	signedTransactionJWS = strings.TrimSpace(signedTransactionJWS)
	if signedTransactionJWS == "" {
		return Transaction{}, ErrVerifyTransaction
	}
	if err := v.validateConfig(); err != nil {
		return Transaction{}, err
	}

	timeout := v.cfg.TimeoutDuration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request := VerifyTransactionRequest{
		SignedTransactionJWS: signedTransactionJWS,
		BundleID:             v.cfg.BundleID,
		Environment:          v.cfg.Environment,
		AppAppleID:           v.cfg.AppAppleID,
		EnableOnlineChecks:   v.cfg.EnableOnlineChecks,
		RootCertificatePaths: v.cfg.RootCertificatePaths,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return Transaction{}, fmt.Errorf("%w: marshal request: %v", ErrVerifyTransaction, err)
	}

	cmd := exec.CommandContext(ctx, v.cfg.NodePath, v.cfg.VerifierScriptPath)
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
		return Transaction{}, fmt.Errorf("%w: %s", ErrVerifyTransaction, message)
	}

	var response VerifyTransactionResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return Transaction{}, fmt.Errorf("%w: decode response: %v", ErrVerifyTransaction, err)
	}
	if !response.OK {
		return Transaction{}, fmt.Errorf("%w: %s", ErrVerifyTransaction, response.Error)
	}

	return response.Transaction, nil
}

func (v *Verifier) validateConfig() error {
	if strings.TrimSpace(v.cfg.NodePath) == "" ||
		strings.TrimSpace(v.cfg.VerifierScriptPath) == "" ||
		strings.TrimSpace(v.cfg.BundleID) == "" ||
		strings.TrimSpace(v.cfg.Environment) == "" ||
		len(v.cfg.RootCertificatePaths) == 0 {
		return ErrVerifierConfigInvalid
	}
	return nil
}
