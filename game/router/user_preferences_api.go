package router

import (
	"context"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

// UserPreferencesApi implements user appearance preference APIs.
type UserPreferencesApi struct {
	pb.UnimplementedUserPreferencesServiceServer
}

func (a *UserPreferencesApi) SaveUserPreferences(ctx context.Context, req *pb.SaveUserPreferencesRequest) (*pb.SaveUserPreferencesResponse, error) {
	uid := session.GetUser(ctx).UID()
	prefs := req.GetPreferences()
	if prefs == nil {
		return session.Error(ctx, gamecode.UserPreferencesInvalid, &pb.SaveUserPreferencesResponse{})
	}

	record, err := mysqlmodel.SaveUserPreferences(uid, prefs)
	if err != nil {
		return session.Error(ctx, gamecode.UserPreferencesSaveFailed, &pb.SaveUserPreferencesResponse{})
	}

	return &pb.SaveUserPreferencesResponse{Preferences: mysqlmodel.UserPreferencesToPB(record)}, nil
}
