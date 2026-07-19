package router

import (
	"context"
	"time"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

type PaywallAnalyticsApi struct {
	pb.UnimplementedPaywallAnalyticsServiceServer
}

func (a *PaywallAnalyticsApi) RecordPaywallSession(ctx context.Context, req *pb.RecordPaywallSessionRequest) (*pb.RecordPaywallSessionResponse, error) {
	if req.GetPresentationId() == "" || req.GetEntryPoint() == "" || req.GetPresentedAt() <= 0 {
		return session.Error(ctx, gamecode.PaywallSessionInvalid, &pb.RecordPaywallSessionResponse{})
	}

	status := mysqlmodel.PaywallSessionStatusDefault
	switch req.GetStatus() {
	case pb.PaywallSessionStatus_PAYWALL_SESSION_STATUS_CANCELLED:
		status = mysqlmodel.PaywallSessionStatusCancelled
	case pb.PaywallSessionStatus_PAYWALL_SESSION_STATUS_PURCHASED:
		status = mysqlmodel.PaywallSessionStatusPurchased
	}

	if err := mysqlmodel.RecordPaywallSession(mysqlmodel.PaywallSessionWrite{
		PresentationID:  req.GetPresentationId(),
		UID:             req.GetPresentedUid(),
		AnonymousID:     req.GetAnonymousId(),
		DeviceUniqueID:  req.GetDeviceUniqueId(),
		EntryPoint:      req.GetEntryPoint(),
		PresentedAt:     unixMillisTime(req.GetPresentedAt()),
		Status:          status,
		StatusChangedAt: unixMillisTime(req.GetStatusChangedAt()),
		ProductID:       req.GetProductId(),
		AppVersion:      req.GetAppVersion(),
	}); err != nil {
		return session.Error(ctx, gamecode.PaywallSessionSaveFailed, &pb.RecordPaywallSessionResponse{})
	}
	return &pb.RecordPaywallSessionResponse{Success: true}, nil
}

func unixMillisTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}
