package router

import (
	"context"
	"errors"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

// FeedbackApi 实现用户反馈相关 gRPC 接口。
type FeedbackApi struct {
	pb.UnimplementedFeedbackServiceServer
}

// SubmitFeedback 提交用户反馈。
func (a *FeedbackApi) SubmitFeedback(ctx context.Context, req *pb.SubmitFeedbackRequest) (*pb.SubmitFeedbackResponse, error) {
	uid := session.GetUser(ctx).UID()

	_, usedToday, err := mysqlmodel.CreateUserFeedback(uid, req.GetContent())
	if errors.Is(err, mysqlmodel.ErrFeedbackContentEmpty) {
		return session.Error(ctx, gamecode.FeedbackContentEmpty, &pb.SubmitFeedbackResponse{})
	}
	if errors.Is(err, mysqlmodel.ErrFeedbackContentTooLong) {
		return session.Error(ctx, gamecode.FeedbackContentTooLong, &pb.SubmitFeedbackResponse{})
	}
	if errors.Is(err, mysqlmodel.ErrFeedbackDailyCreateLimitExceeded) {
		return session.Error(ctx, gamecode.FeedbackDailyLimitExceeded, &pb.SubmitFeedbackResponse{
			UsedToday:  int32(usedToday),
			DailyLimit: int32(mysqlmodel.MaxFeedbackCreatesPerDay),
		})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FeedbackSaveFailed, &pb.SubmitFeedbackResponse{})
	}

	return &pb.SubmitFeedbackResponse{
		Success:    true,
		UsedToday:  int32(usedToday),
		DailyLimit: int32(mysqlmodel.MaxFeedbackCreatesPerDay),
	}, nil
}
