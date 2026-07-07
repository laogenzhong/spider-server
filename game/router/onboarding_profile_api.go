package router

import (
	"context"
	"encoding/json"
	"time"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

// OnboardingProfileApi implements onboarding profile upload APIs.
type OnboardingProfileApi struct {
	pb.UnimplementedOnboardingProfileServiceServer
}

func (a *OnboardingProfileApi) UploadOnboardingProfile(ctx context.Context, req *pb.UploadOnboardingProfileRequest) (*pb.UploadOnboardingProfileResponse, error) {
	uid := session.GetUser(ctx).UID()
	profileJSON := []byte(req.GetProfileJson())
	if len(profileJSON) == 0 || !json.Valid(profileJSON) {
		return session.Error(ctx, gamecode.OnboardingProfileInvalid, &pb.UploadOnboardingProfileResponse{})
	}

	schemaVersion := int(req.GetSchemaVersion())
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	completedAt := time.Now()
	if req.GetCompletedAt() > 0 {
		completedAt = time.UnixMilli(req.GetCompletedAt())
	}

	if err := mysqlmodel.SaveOnboardingProfile(uid, profileJSON, schemaVersion, completedAt); err != nil {
		return session.Error(ctx, gamecode.OnboardingProfileSaveFailed, &pb.UploadOnboardingProfileResponse{})
	}

	return &pb.UploadOnboardingProfileResponse{Success: true}, nil
}
