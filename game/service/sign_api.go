package service

import (
	"context"
	"time"

	api "spider-server/gen/spider/api"

	"google.golang.org/protobuf/types/known/emptypb"
)

type SignApi struct {
	api.UnimplementedSignApiServer
}

func (s *SignApi) SignIn(ctx context.Context, req *api.SignInRequest) (*api.SignInResponse, error) {
	return &api.SignInResponse{}, nil
}

func (s *SignApi) SignUpMixed(ctx context.Context, req *api.SignInRequest) (*api.SignUpMixedResponse, error) {
	return &api.SignUpMixedResponse{}, nil
}

func (s *SignApi) Token(ctx context.Context, req *api.TokenRequest) (*api.SignInResponse, error) {
	return &api.SignInResponse{}, nil
}

func (s *SignApi) SignOut(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	_ = time.Now()
	return &emptypb.Empty{}, nil
}
