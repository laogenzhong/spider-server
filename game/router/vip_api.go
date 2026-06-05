package router

import (
	"context"
	"errors"
	"strings"
	"time"

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
		return session.Error(ctx, gamecode.VIPTransactionVerifyConfigInvalid, &pb.VIPStatusResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.VIPTransactionVerifyFailed, &pb.VIPStatusResponse{})
	}

	if requestProductID := strings.TrimSpace(req.GetProductId()); requestProductID != "" && requestProductID != transaction.ProductID {
		return session.Error(ctx, gamecode.VIPProductUnsupported, &pb.VIPStatusResponse{})
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
