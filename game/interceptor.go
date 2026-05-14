package game

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	"strings"
)

// publicGRPCMethodPrefixes 配置不需要登录拦截的 gRPC 方法前缀。
//
// gRPC 的 info.FullMethod 格式一般是：
// /包名.服务名/方法名
// 例如：
// /api.uc.SignApi/SignIn
// /api.WeightRecordService/CreateWeightRecord
//
// 配置规则：
// 1. 默认所有接口都需要拦截校验。
// 2. 只有命中这里配置的前缀才会跳过拦截。
// 3. 例如配置 "/api.uc."，那么所有 /api.uc.* 服务都不会拦截。
var publicGRPCMethodPrefixes = []string{
	"/uc.",
}

// shouldSkipAuthInterceptor 判断当前 gRPC 方法是否跳过登录拦截。
func shouldSkipAuthInterceptor(fullMethod string) bool {
	for _, prefix := range publicGRPCMethodPrefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(fullMethod, prefix) {
			return true
		}
	}
	return false
}

// authUnaryInterceptor 是一元 gRPC 拦截器。
//
// 默认拦截所有接口；
// 只有命中 publicGRPCMethodPrefixes 的接口才会直接放行，例如登录、注册、调试接口。
func authUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	if shouldSkipAuthInterceptor(info.FullMethod) {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		// 缺少请求 metadata
		return session.Error2(ctx, gamecode.MdNull)
	}

	tokens := md.Get("xx-token")
	if len(tokens) == 0 || strings.TrimSpace(tokens[0]) == "" {
		return session.Error2(ctx, gamecode.SessionNull)
	}

	user := session.FindUser(ctx)
	err := user.Check()
	if err != nil {
		return session.Error2(ctx, gamecode.SessionExpire)
	}

	ctx = session.WithUser(ctx, user)

	return handler(ctx, req)
}
