package router

import (
	"context"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

// WeeklyTrainingGoalApi 实现每周训练目标相关 gRPC 接口。
type WeeklyTrainingGoalApi struct {
	pb.UnimplementedWeeklyTrainingGoalServiceServer
}

func (a *WeeklyTrainingGoalApi) GetWeeklyTrainingGoal(ctx context.Context, req *pb.GetWeeklyTrainingGoalRequest) (*pb.GetWeeklyTrainingGoalResponse, error) {
	uid := session.GetUser(ctx).UID()

	goal, err := mysqlmodel.GetWeeklyTrainingGoal(uid)
	if err != nil {
		return session.Error(ctx, gamecode.WeeklyTrainingGoalQueryFailed, &pb.GetWeeklyTrainingGoalResponse{})
	}

	return &pb.GetWeeklyTrainingGoalResponse{Goal: mysqlmodel.WeeklyTrainingGoalToPB(goal)}, nil
}

func (a *WeeklyTrainingGoalApi) SaveWeeklyTrainingGoal(ctx context.Context, req *pb.SaveWeeklyTrainingGoalRequest) (*pb.SaveWeeklyTrainingGoalResponse, error) {
	uid := session.GetUser(ctx).UID()

	if !isValidWeeklyTrainingGoal(req.GetStrengthSessions()) ||
		!isValidWeeklyTrainingGoal(req.GetCardioSessions()) ||
		!mysqlmodel.HasValidWeeklyTrainingGoalTotal(req.GetStrengthSessions(), req.GetCardioSessions()) {
		return session.Error(ctx, gamecode.WeeklyTrainingGoalInvalid, &pb.SaveWeeklyTrainingGoalResponse{})
	}

	goal, err := mysqlmodel.SaveWeeklyTrainingGoal(uid, req.GetStrengthSessions(), req.GetCardioSessions())
	if err != nil {
		return session.Error(ctx, gamecode.WeeklyTrainingGoalSaveFailed, &pb.SaveWeeklyTrainingGoalResponse{})
	}

	return &pb.SaveWeeklyTrainingGoalResponse{Goal: mysqlmodel.WeeklyTrainingGoalToPB(goal)}, nil
}

func isValidWeeklyTrainingGoal(value int32) bool {
	return value >= 0 && value <= mysqlmodel.MaxWeeklyTrainingGoal
}
