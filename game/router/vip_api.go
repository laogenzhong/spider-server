package router

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	appconfig "spider-server/common/config"
	"spider-server/game/appstore"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

type VIPApi struct {
	pb.UnimplementedVIPServiceServer
}

func (s *VIPApi) GetVIPStatus(ctx context.Context, req *pb.GetVIPStatusRequest) (*pb.VIPStatusResponse, error) {
	return s.currentStatusResponse(ctx)
}

func (s *VIPApi) CreateApplePurchaseOrder(ctx context.Context, req *pb.CreateApplePurchaseOrderRequest) (*pb.CreateApplePurchaseOrderResponse, error) {
	user := session.GetUser(ctx)
	if user == nil || user.UIDOrDefault() == 0 {
		return session.Error(ctx, gamecode.SessionNull, &pb.CreateApplePurchaseOrderResponse{})
	}

	productID := strings.TrimSpace(req.GetProductId())
	if productID == "" {
		return session.Error(ctx, gamecode.VIPProductUnsupported, &pb.CreateApplePurchaseOrderResponse{})
	}

	verifier := appstore.DefaultVerifier()
	cfg := verifier.Config()
	now := time.Now()
	order, err := mysqlmodel.CreateApplePurchaseOrder(
		user.UIDOrDefault(),
		productID,
		cfg.MonthlyProductID,
		cfg.LifetimeProductID,
		now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported product id") {
			return session.Error(ctx, gamecode.VIPProductUnsupported, &pb.CreateApplePurchaseOrderResponse{})
		}
		return session.Error(ctx, gamecode.VIPPurchaseOrderCreateFailed, &pb.CreateApplePurchaseOrderResponse{})
	}

	return &pb.CreateApplePurchaseOrderResponse{
		OrderId:    order.OrderID,
		ProductId:  order.ProductID,
		ExpiresAt:  order.ExpiresAt.Unix(),
		ServerTime: now.Unix(),
	}, nil
}

func (s *VIPApi) ConfirmAppleTransaction(ctx context.Context, req *pb.ConfirmAppleTransactionRequest) (*pb.VIPStatusResponse, error) {
	if strings.TrimSpace(req.GetSignedTransactionJws()) == "" {
		return session.Error(ctx, gamecode.VIPTransactionJWSMissing, &pb.VIPStatusResponse{})
	}
	if strings.TrimSpace(req.GetOrderId()) == "" {
		return session.Error(ctx, gamecode.VIPPurchaseOrderRequired, &pb.VIPStatusResponse{})
	}

	user := session.GetUser(ctx)
	if user == nil || user.UIDOrDefault() == 0 {
		return session.Error(ctx, gamecode.SessionNull, &pb.VIPStatusResponse{})
	}

	verifier := appstore.DefaultVerifier()
	transaction, err := verifier.VerifyTransaction(ctx, req.GetSignedTransactionJws())
	if errors.Is(err, appstore.ErrVerifierConfigInvalid) {
		logAppleTransactionVerifyFailure(user.UIDOrDefault(), req, verifier.Config(), err)
		return session.Error(ctx, gamecode.VIPTransactionVerifyConfigInvalid, &pb.VIPStatusResponse{})
	}
	if err != nil {
		logAppleTransactionVerifyFailure(user.UIDOrDefault(), req, verifier.Config(), err)
		return session.Error(ctx, gamecode.VIPTransactionVerifyFailed, &pb.VIPStatusResponse{})
	}

	if requestProductID := strings.TrimSpace(req.GetProductId()); requestProductID == "" || requestProductID != strings.TrimSpace(transaction.ProductID) {
		return session.Error(ctx, gamecode.VIPProductUnsupported, &pb.VIPStatusResponse{})
	}
	if requestTransactionID := strings.TrimSpace(req.GetTransactionId()); requestTransactionID == "" || requestTransactionID != strings.TrimSpace(transaction.TransactionID) {
		return session.Error(ctx, gamecode.VIPPurchaseOrderTransactionMismatch, &pb.VIPStatusResponse{})
	}
	if requestOriginalTransactionID := strings.TrimSpace(req.GetOriginalTransactionId()); requestOriginalTransactionID == "" || requestOriginalTransactionID != strings.TrimSpace(transaction.OriginalTransactionID) {
		return session.Error(ctx, gamecode.VIPPurchaseOrderTransactionMismatch, &pb.VIPStatusResponse{})
	}

	cfg := verifier.Config()
	if err := mysqlmodel.SaveAppleTransactionAndGrantVIP(
		user.UIDOrDefault(),
		req.GetOrderId(),
		transaction,
		req.GetSignedTransactionJws(),
		cfg.MonthlyProductID,
		cfg.LifetimeProductID,
		time.Now(),
	); err != nil {
		if strings.Contains(err.Error(), "unsupported product id") {
			return session.Error(ctx, gamecode.VIPProductUnsupported, &pb.VIPStatusResponse{})
		}
		if errors.Is(err, mysqlmodel.ErrApplePurchaseOrderNotFound) {
			return session.Error(ctx, gamecode.VIPPurchaseOrderMissing, &pb.VIPStatusResponse{})
		}
		if errors.Is(err, mysqlmodel.ErrApplePurchaseOrderExpired) {
			return session.Error(ctx, gamecode.VIPPurchaseOrderExpired, &pb.VIPStatusResponse{})
		}
		if errors.Is(err, mysqlmodel.ErrApplePurchaseOrderProductMismatch) {
			return session.Error(ctx, gamecode.VIPPurchaseOrderProductMismatch, &pb.VIPStatusResponse{})
		}
		if errors.Is(err, mysqlmodel.ErrApplePurchaseOrderTransactionMismatch) {
			return session.Error(ctx, gamecode.VIPPurchaseOrderTransactionMismatch, &pb.VIPStatusResponse{})
		}
		if errors.Is(err, mysqlmodel.ErrAppleTransactionOwnedByOtherUser) {
			return session.Error(ctx, gamecode.VIPAppleTransactionAlreadyBound, &pb.VIPStatusResponse{})
		}
		return session.Error(ctx, gamecode.VIPEntitlementSaveFailed, &pb.VIPStatusResponse{})
	}

	return s.currentStatusResponse(ctx)
}

func (s *VIPApi) currentStatusResponse(ctx context.Context) (*pb.VIPStatusResponse, error) {
	user := session.GetUser(ctx)
	if user == nil || user.UIDOrDefault() == 0 {
		return session.Error(ctx, gamecode.SessionNull, &pb.VIPStatusResponse{})
	}

	now := time.Now()
	status, err := mysqlmodel.GetCurrentVIPStatus(user.UIDOrDefault(), now)
	if err != nil {
		return session.Error(ctx, gamecode.VIPStatusQueryFailed, &pb.VIPStatusResponse{})
	}

	return &pb.VIPStatusResponse{
		Status: toPBVIPStatus(status, now),
	}, nil
}

func toPBVIPStatus(status mysqlmodel.CurrentVIPStatus, now time.Time) *pb.VIPStatus {
	var expiresAt int64
	if status.ExpiresAt != nil {
		expiresAt = status.ExpiresAt.Unix()
	}

	return &pb.VIPStatus{
		IsVip:      status.IsVIP,
		Kind:       toPBVIPKind(status.Kind),
		ExpiresAt:  expiresAt,
		ProductId:  status.ProductID,
		Source:     status.Source,
		ServerTime: now.Unix(),
	}
}

func toPBVIPKind(kind string) pb.VIPKind {
	switch kind {
	case mysqlmodel.VIPKindLifetime:
		return pb.VIPKind_VIP_KIND_LIFETIME
	case mysqlmodel.VIPKindMonthly:
		return pb.VIPKind_VIP_KIND_MONTHLY
	default:
		return pb.VIPKind_VIP_KIND_NONE
	}
}

func logAppleTransactionVerifyFailure(uid uint64, req *pb.ConfirmAppleTransactionRequest, cfg appconfig.AppStoreConfig, err error) {
	jws := strings.TrimSpace(req.GetSignedTransactionJws())
	log.Printf(
		"apple transaction verify failed uid=%d order=%s product=%s transaction=%s original=%s bundle=%s environment=%s appAppleId=%d onlineChecks=%t rootCerts=%d jwsLength=%d jwsDots=%d err=%v",
		uid,
		req.GetOrderId(),
		req.GetProductId(),
		req.GetTransactionId(),
		req.GetOriginalTransactionId(),
		cfg.BundleID,
		cfg.Environment,
		cfg.AppAppleID,
		cfg.EnableOnlineChecks,
		len(cfg.RootCertificatePaths),
		len(jws),
		strings.Count(jws, "."),
		err,
	)
	severity := mysqlmodel.ApplePaymentFailureSeverityWarning
	errorCode := gamecode.VIPTransactionVerifyFailed
	problem := "Apple transaction JWS verification failed, so the VIP entitlement was not granted."
	if errors.Is(err, appstore.ErrVerifierConfigInvalid) {
		severity = mysqlmodel.ApplePaymentFailureSeverityCritical
		errorCode = gamecode.VIPTransactionVerifyConfigInvalid
		problem = "Apple transaction verifier configuration is invalid, so all transaction confirmations may fail until the server config is fixed."
	}
	mysqlmodel.RecordApplePaymentFailureBestEffort(mysqlmodel.ApplePaymentFailure{
		Category:              mysqlmodel.ApplePaymentFailureCategoryTransactionVerify,
		Stage:                 mysqlmodel.ApplePaymentFailureStageTransactionVerify,
		Severity:              severity,
		UID:                   uid,
		OrderID:               req.GetOrderId(),
		ProductID:             req.GetProductId(),
		TransactionID:         req.GetTransactionId(),
		OriginalTransactionID: req.GetOriginalTransactionId(),
		BundleID:              cfg.BundleID,
		Environment:           cfg.Environment,
		ErrorCode:             errorCode,
		Reason:                errString(err),
		Problem:               problem,
		ErrorMessage:          errString(err),
		ContextJSON: mysqlmodel.ApplePaymentFailureContext(map[string]any{
			"appAppleID":         cfg.AppAppleID,
			"enableOnlineChecks": cfg.EnableOnlineChecks,
			"rootCertCount":      len(cfg.RootCertificatePaths),
			"jwsLength":          len(jws),
			"jwsDots":            strings.Count(jws, "."),
			"nodePath":           cfg.NodePath,
			"verifierScriptPath": cfg.VerifierScriptPath,
		}),
		OccurredAt: time.Now(),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
