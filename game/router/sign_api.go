package router

import (
	"context"
	"errors"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	"spider-server/mysql/model"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
	api "spider-server/gen/spider/api"
)

type SignApi struct {
	api.UnimplementedSignApiServer
}

const sessionAttachAccountKey = "account"

func (s *SignApi) SignIn(ctx context.Context, req *api.SignInRequest) (*api.SignInResponse, error) {
	account := req.GetAccount()
	password := req.GetPwd()

	if account == "" {
		return session.Error(ctx, gamecode.SignAccountEmpty, &api.SignInResponse{})
	}

	if password == "" {
		return session.Error(ctx, gamecode.SignPasswordEmpty, &api.SignInResponse{})
	}

	user, err := mysqlmodel.GetUserByAccount(account)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_, err = mysqlmodel.CreateUser(account, password)
		user, err = mysqlmodel.GetUserByAccount(account)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return session.Error(ctx, gamecode.SignAccountNotFound, &api.SignInResponse{})
		}
	}
	if err != nil {
		return session.Error(ctx, gamecode.SignQueryAccountFailed, &api.SignInResponse{})
	}

	if user.Password != password {
		return session.Error(ctx, gamecode.SignPasswordWrong, &api.SignInResponse{})
	}

	token, _, err := session.SignSessionManager.NewToken(ctx, uint64(user.ID), 1, map[string]string{
		sessionAttachAccountKey: user.Account,
	})
	if err != nil {
		return session.Error(ctx, gamecode.SignCreateTokenFailed, &api.SignInResponse{})
	}

	resp := &api.SignInResponse{}
	resp.Uid = uint64(user.ID)
	resp.UcToken = token

	return resp, nil
}

func (s *SignApi) SignUpMixed(ctx context.Context, req *api.SignInRequest) (*api.SignUpMixedResponse, error) {
	account := req.GetAccount()
	password := req.GetPwd()

	if account == "" {
		return session.Error(ctx, gamecode.SignAccountEmpty, &api.SignUpMixedResponse{})
	}

	if password == "" {
		return session.Error(ctx, gamecode.SignPasswordEmpty, &api.SignUpMixedResponse{})
	}

	_, err := mysqlmodel.GetUserByAccount(account)
	if err == nil {
		return session.Error(ctx, gamecode.SignAccountExists, &api.SignUpMixedResponse{})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.SignQueryAccountFailed, &api.SignUpMixedResponse{})
	}

	_, err = mysqlmodel.CreateUser(account, password)
	if err != nil {
		return session.Error(ctx, gamecode.SignCreateUserFailed, &api.SignUpMixedResponse{})
	}

	return &api.SignUpMixedResponse{}, nil
}

func (s *SignApi) Token(ctx context.Context, req *api.TokenRequest) (*api.SignInResponse, error) {
	oldToken := session.GetTokenFromContext(ctx)
	if oldToken == "" {
		return session.Error(ctx, gamecode.SignTokenEmpty, &api.SignInResponse{})
	}

	user, err := session.SignSessionManager.FromToken(ctx, oldToken, sessionAttachAccountKey)
	if err != nil {
		return session.Error(ctx, gamecode.SignTokenInvalid, &api.SignInResponse{})
	}

	uid := user.UIDOrDefault()
	scopeID, err := user.ScopeID()
	if err != nil {
		return session.Error(ctx, gamecode.SignTokenInvalid, &api.SignInResponse{})
	}

	account, _ := user.GetAttachString(sessionAttachAccountKey)
	newToken, _, err := session.SignSessionManager.NewToken(ctx, uid, scopeID, map[string]string{
		sessionAttachAccountKey: account,
	})
	if err != nil {
		return session.Error(ctx, gamecode.SignRefreshTokenFailed, &api.SignInResponse{})
	}

	resp := &api.SignInResponse{}
	resp.Uid = uid
	resp.UcToken = newToken
	return resp, nil
}

func (s *SignApi) SignOut(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	oldToken := session.GetTokenFromContext(ctx)
	if oldToken == "" {
		return session.Error(ctx, gamecode.SignTokenEmpty, &emptypb.Empty{})
	}

	user, err := session.SignSessionManager.FromToken(ctx, oldToken, sessionAttachAccountKey)
	if err != nil {
		return &emptypb.Empty{}, nil
		//return session.Error(ctx, gamecode.SignTokenInvalid, &emptypb.Empty{})
	}

	if err := user.Logout(ctx); err != nil {
		return session.Error(ctx, gamecode.SignLogoutFailed, &emptypb.Empty{})
	}

	return &emptypb.Empty{}, nil
}
