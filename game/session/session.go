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

func GetTokenFromContext(ctx context.Context) string {
	token := GetIncomingValue(ctx, "xx-token")
	if token != "" {
		return token
	}

	return ""
}
