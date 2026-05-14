package session

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strconv"
)

func SetHeader(ctx context.Context, key string, value string) error {
	return grpc.SetHeader(ctx, metadata.Pairs(key, value))
}

func SetHeaders(ctx context.Context, headers map[string]string) error {
	if len(headers) == 0 {
		return nil
	}

	pairs := make([]string, 0, len(headers)*2)
	for key, value := range headers {
		if key == "" {
			continue
		}
		pairs = append(pairs, key, value)
	}

	if len(pairs) == 0 {
		return nil
	}
	return grpc.SetHeader(ctx, metadata.Pairs(pairs...))
}

func SetTrailer(ctx context.Context, key string, value string) error {
	return grpc.SetTrailer(ctx, metadata.Pairs(key, value))
}

func SetTrailers(ctx context.Context, trailers map[string]string) error {
	if len(trailers) == 0 {
		return nil
	}

	pairs := make([]string, 0, len(trailers)*2)
	for key, value := range trailers {
		if key == "" {
			continue
		}
		pairs = append(pairs, key, value)
	}

	if len(pairs) == 0 {
		return nil
	}
	return grpc.SetTrailer(ctx, metadata.Pairs(pairs...))
}

func IncomingMetadata(ctx context.Context) metadata.MD {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return metadata.MD{}
	}
	return md
}

func GetIncomingValue(ctx context.Context, key string) string {
	values := IncomingMetadata(ctx).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func GetIncomingValues(ctx context.Context, key string) []string {
	values := IncomingMetadata(ctx).Get(key)
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func Error[T any](ctx context.Context, value int, data T) (T, error) {
	strV := strconv.Itoa(value)
	if err := grpc.SetTrailer(ctx, metadata.Pairs("status_code", strV)); err != nil {
		return data, status.Error(codes.InvalidArgument, "set err err")
	}
	return data, nil
}

func Error2(ctx context.Context, value int) (any, error) {
	strV := strconv.Itoa(value)
	if err := grpc.SetTrailer(ctx, metadata.Pairs("status_code", strV)); err != nil {
		return nil, status.Error(codes.InvalidArgument, "set err err")
	}
	return nil, nil
}

func GetTokenFromContext(ctx context.Context) string {
	token := GetIncomingValue(ctx, "xx-token")
	if token != "" {
		return token
	}

	return ""
}

func FindUser(ctx context.Context) *SessionUser {
	token := GetIncomingValue(ctx, "xx-token")
	if token != "" {
		user, _ := SignSessionManager.FromToken(ctx, token)
		return user
	}

	return nil
}

type userContextKey struct{}

func WithUser(ctx context.Context, user *SessionUser) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// 虽然这两个 userContextKey{} 是两个新创建的空对象，但它们的类型和值都相同，所以可以匹配上。
// 1. 不占额外空间
//2. 不容易冲突
//3. 只能在当前包里直接使用
//4. Go 官方文档也推荐不要用普通 string 作为 context key

// 字符串容易冲突

func GetUser(ctx context.Context) *SessionUser {
	user, _ := ctx.Value(userContextKey{}).(*SessionUser)
	return user

}
