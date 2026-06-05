package router

import (
	"context"
	"errors"
	"spider-server/game/appleauth"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	"spider-server/mysql/model"
	"time"

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

func (s *SignApi) SignInWithApple(ctx context.Context, req *api.AppleSignInRequest) (*api.SignInResponse, error) {
	identityToken := req.GetIdentityToken()
	if identityToken == "" {
		return session.Error(ctx, gamecode.SignAppleIdentityTokenEmpty, &api.SignInResponse{})
	}

	client := appleauth.DefaultClient()
	claims, err := client.VerifyIdentityToken(ctx, identityToken, req.GetNonce())
	if err != nil {
		return appleSignInError(ctx, err, &api.SignInResponse{})
	}

	tokenResp, err := client.ExchangeAuthorizationCode(ctx, req.GetAuthorizationCode())
	if err != nil {
		return appleSignInError(ctx, err, &api.SignInResponse{})
	}

	email := claims.Email
	if email == "" {
		email = req.GetEmail()
	}

	profile := mysqlmodel.AppleSignInProfile{
		AppleSub:       claims.Subject,
		Email:          email,
		EmailVerified:  claims.EmailVerified,
		IsPrivateEmail: claims.IsPrivateEmail,
		FullName:       req.GetFullName(),
	}
	if tokenResp != nil {
		profile.RefreshToken = tokenResp.RefreshToken
		profile.AccessToken = tokenResp.AccessToken
		profile.IDToken = tokenResp.IDToken
		profile.ExpiresIn = tokenResp.ExpiresIn
	}
	if profile.IDToken == "" {
		profile.IDToken = identityToken
	}

	user, err := mysqlmodel.FindOrCreateUserForAppleSignIn(profile, mysqlmodel.AppleGeneratedAccount(claims.Subject))
	if err != nil {
		return session.Error(ctx, gamecode.SignAppleAccountBindFailed, &api.SignInResponse{})
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

func (s *SignApi) DeleteAccount(ctx context.Context, req *api.DeleteAccountRequest) (*api.DeleteAccountResponse, error) {
	oldToken := session.GetTokenFromContext(ctx)
	if oldToken == "" {
		return session.Error(ctx, gamecode.SignTokenEmpty, &api.DeleteAccountResponse{})
	}

	user, err := session.SignSessionManager.FromToken(ctx, oldToken, sessionAttachAccountKey)
	if err != nil {
		return session.Error(ctx, gamecode.SignTokenInvalid, &api.DeleteAccountResponse{})
	}

	uid := user.UIDOrDefault()
	if uid == 0 {
		return session.Error(ctx, gamecode.SignTokenInvalid, &api.DeleteAccountResponse{})
	}

	if err := deleteAppleSignInBindingIfNeeded(ctx, uint(uid), req.GetRevokeAppleSignIn(), req.GetReason()); err != nil {
		if isAppleAuthError(err) {
			return appleSignInError(ctx, err, &api.DeleteAccountResponse{})
		}
		return session.Error(ctx, gamecode.SignDeleteAccountFailed, &api.DeleteAccountResponse{})
	}

	if err := mysqlmodel.MarkUserAccountDeletedByID(uint(uid)); err != nil {
		return session.Error(ctx, gamecode.SignDeleteAccountFailed, &api.DeleteAccountResponse{})
	}

	if err := user.Logout(ctx); err != nil {
		return session.Error(ctx, gamecode.SignLogoutFailed, &api.DeleteAccountResponse{})
	}

	return &api.DeleteAccountResponse{
		Success: true,
		Message: "ok",
	}, nil
}

func isAppleAuthError(err error) bool {
	return errors.Is(err, appleauth.ErrIdentityTokenEmpty) ||
		errors.Is(err, appleauth.ErrIdentityTokenInvalid) ||
		errors.Is(err, appleauth.ErrNonceInvalid) ||
		errors.Is(err, appleauth.ErrConfigInvalid) ||
		errors.Is(err, appleauth.ErrTokenExchangeFailed) ||
		errors.Is(err, appleauth.ErrTokenRevokeFailed)
}

func deleteAppleSignInBindingIfNeeded(ctx context.Context, uid uint, revokeAppleSignIn bool, reason string) error {
	account, err := mysqlmodel.GetAppleSignInAccountByUserID(uid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if revokeAppleSignIn {
		revokeToken := account.RefreshToken
		tokenTypeHint := "refresh_token"
		if revokeToken == "" {
			revokeToken = account.AccessToken
			tokenTypeHint = "access_token"
		}
		if err := appleauth.DefaultClient().RevokeToken(ctx, revokeToken, tokenTypeHint); err != nil {
			return err
		}
	}

	return mysqlmodel.ArchiveAndDeleteAppleSignInAccount(account, time.Now(), revokeAppleSignIn, reason)
}

func appleSignInError[T any](ctx context.Context, err error, response T) (T, error) {
	switch {
	case errors.Is(err, appleauth.ErrIdentityTokenEmpty):
		return session.Error(ctx, gamecode.SignAppleIdentityTokenEmpty, response)
	case errors.Is(err, appleauth.ErrNonceInvalid):
		return session.Error(ctx, gamecode.SignAppleNonceInvalid, response)
	case errors.Is(err, appleauth.ErrConfigInvalid):
		return session.Error(ctx, gamecode.SignAppleConfigInvalid, response)
	case errors.Is(err, appleauth.ErrTokenExchangeFailed):
		return session.Error(ctx, gamecode.SignAppleTokenExchangeFailed, response)
	case errors.Is(err, appleauth.ErrTokenRevokeFailed):
		return session.Error(ctx, gamecode.SignAppleTokenRevokeFailed, response)
	default:
		return session.Error(ctx, gamecode.SignAppleIdentityTokenInvalid, response)
	}
}
