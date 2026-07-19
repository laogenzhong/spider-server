package game

import (
	"google.golang.org/grpc"
	appconfig "spider-server/common/config"
	applogger "spider-server/common/logger"
	"spider-server/game/router"
	pb "spider-server/gen/spider/api"
)

func ConfigureWorkoutDataSync(cfg appconfig.WorkoutDataSyncConfig) {
	router.ConfigureWorkoutDataSyncLimits(cfg)
}

func (s *GRPCServer) Init() {
	if err := s.Register(func(server *grpc.Server) {
		pb.RegisterRoomSyncApiServer(server, &router.RoomSyncApi{})
		pb.RegisterSignApiServer(server, &router.SignApi{})
		pb.RegisterWeightRecordServiceServer(server, &router.WeightApi{})
		pb.RegisterWeeklyTrainingGoalServiceServer(server, &router.WeeklyTrainingGoalApi{})
		pb.RegisterOnboardingProfileServiceServer(server, &router.OnboardingProfileApi{})
		pb.RegisterUserPreferencesServiceServer(server, &router.UserPreferencesApi{})
		pb.RegisterExerciseSetRecordServiceServer(server, &router.ExerciseSetRecordApi{})
		pb.RegisterTrainingTagServiceServer(server, &router.TrainingTagApi{})
		pb.RegisterClientRestoreServiceServer(server, &router.ClientRestoreApi{})
		pb.RegisterBodyPhotoServiceServer(server, &router.BodyPhotoApi{})
		pb.RegisterFeedbackServiceServer(server, &router.FeedbackApi{})
		pb.RegisterFriendServiceServer(server, &router.FriendApi{})
		pb.RegisterVIPServiceServer(server, &router.VIPApi{})
		pb.RegisterAppUpdateServiceServer(server, &router.AppUpdateApi{})
		pb.RegisterAdminVIPApiServer(server, &router.AdminVIPApi{})
		pb.RegisterClientSyncFailureServiceServer(server, &router.ClientSyncFailureApi{})
	}); err != nil {
		applogger.Fatalf("register room sync grpc router failed: %v", err)
	}
}
