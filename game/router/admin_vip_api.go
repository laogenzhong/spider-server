package router

import (
	"context"
	"errors"
	"strings"
	"time"

	appconfig "spider-server/common/config"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/grpc/metadata"
)

type AdminVIPApi struct {
	pb.UnimplementedAdminVIPApiServer
}

func (s *AdminVIPApi) GrantVIP(ctx context.Context, req *pb.AdminGrantVIPRequest) (*pb.AdminGrantVIPResponse, error) {
	if !validAdminSecret(ctx) {
		return session.Error(ctx, gamecode.AdminVIPSecretInvalid, &pb.AdminGrantVIPResponse{})
	}

	account := strings.TrimSpace(req.GetAccount())
	if account == "" {
		return session.Error(ctx, gamecode.AdminVIPAccountEmpty, &pb.AdminGrantVIPResponse{})
	}

	now := time.Now()
	user, status, err := mysqlmodel.GrantAdminVIPByAccount(
		account,
		req.GetLifetime(),
		req.GetDurationDays(),
		req.GetExpiresAt(),
		req.GetOperator(),
		req.GetReason(),
		now,
	)
	if err != nil {
		switch {
		case errors.Is(err, mysqlmodel.ErrAdminVIPAccountNotFound):
			return session.Error(ctx, gamecode.AdminVIPAccountNotFound, &pb.AdminGrantVIPResponse{})
		case errors.Is(err, mysqlmodel.ErrAdminVIPDurationInvalid):
			return session.Error(ctx, gamecode.AdminVIPDurationInvalid, &pb.AdminGrantVIPResponse{})
		default:
			return session.Error(ctx, gamecode.AdminVIPGrantFailed, &pb.AdminGrantVIPResponse{})
		}
	}

	return &pb.AdminGrantVIPResponse{
		Uid:     uint64(user.ID),
		Account: user.Account,
		Status:  toPBVIPStatus(status, now),
	}, nil
}

func (s *AdminVIPApi) RevokeAdminVIP(ctx context.Context, req *pb.AdminRevokeVIPRequest) (*pb.AdminRevokeVIPResponse, error) {
	if !validAdminSecret(ctx) {
		return session.Error(ctx, gamecode.AdminVIPSecretInvalid, &pb.AdminRevokeVIPResponse{})
	}

	account := strings.TrimSpace(req.GetAccount())
	if account == "" {
		return session.Error(ctx, gamecode.AdminVIPAccountEmpty, &pb.AdminRevokeVIPResponse{})
	}

	now := time.Now()
	user, status, err := mysqlmodel.RevokeAdminVIPByAccount(
		account,
		req.GetOperator(),
		req.GetReason(),
		now,
	)
	if err != nil {
		switch {
		case errors.Is(err, mysqlmodel.ErrAdminVIPAccountNotFound):
			return session.Error(ctx, gamecode.AdminVIPAccountNotFound, &pb.AdminRevokeVIPResponse{})
		default:
			return session.Error(ctx, gamecode.AdminVIPRevokeFailed, &pb.AdminRevokeVIPResponse{})
		}
	}

	return &pb.AdminRevokeVIPResponse{
		Uid:     uint64(user.ID),
		Account: user.Account,
		Status:  toPBVIPStatus(status, now),
	}, nil
}

func validAdminSecret(ctx context.Context) bool {
	cfg, err := appconfig.LoadDefault()
	if err != nil {
		return false
	}
	secret := strings.TrimSpace(cfg.Admin.VIPGrantSecret)
	if secret == "" {
		return false
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	values := md.Get("xx-admin-secret")
	if len(values) == 0 {
		return false
	}
	return strings.TrimSpace(values[0]) == secret
}
