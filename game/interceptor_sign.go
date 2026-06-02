package game

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"log"
	"sort"
	"spider-server/game/session"
	"strings"
	"sync"
	"time"
)

var replayNonceCache sync.Map

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func buildServerSignContent(method string, md metadata.MD, bodyBytes []byte) []byte {
	cleanPath := strings.TrimPrefix(strings.TrimSpace(method), "/")

	headerValues := make(map[string][]string)
	for k, values := range md {
		key := strings.ToLower(strings.TrimSpace(k))
		if !strings.HasPrefix(key, "xx-") || key == "xx-sign" {
			continue
		}
		headerValues[key] = append(headerValues[key], values...)
	}

	keys := make([]string, 0, len(headerValues))
	for k := range headerValues {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(cleanPath)
	builder.WriteString("&")

	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString("=")
		builder.WriteString(strings.Join(headerValues[k], ""))
		builder.WriteString("&")
	}

	builder.WriteString(sha256Hex(bodyBytes))
	return []byte(builder.String())
}

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now().Unix()
			replayNonceCache.Range(func(key, value any) bool {
				expireAt, ok := value.(int64)
				if ok && expireAt <= now {
					replayNonceCache.Delete(key)
				}
				return true
			})
		}
	}()
}

// MetadataLogInterceptor 打印请求携带的 metadata
func MetadataLogInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		xxMeta := make(map[string][]string)
		for k, v := range md {
			if strings.HasPrefix(k, "xx-") {
				xxMeta[k] = v
			}
		}

		log.Printf("grpc method=%s metadata=%v", info.FullMethod, xxMeta)
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		signs := md.Get("xx-sign")
		if len(signs) == 0 {
			return nil, fmt.Errorf("missing xx-sign")
		}

		msg, ok := req.(proto.Message)
		if !ok {
			return nil, fmt.Errorf("request is not proto message")
		}

		bodyBytes, err := proto.Marshal(msg)
		if err != nil {
			return nil, err
		}

		canonicalBytes := buildServerSignContent(info.FullMethod, md, bodyBytes)
		expectSign := sha256Hex(canonicalBytes)

		if signs[0] != expectSign {
			return nil, fmt.Errorf("invalid sign")
		}
	}

	uid := session.GetUser(ctx).UID()

	nonce := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("xx-nonce"); len(values) > 0 {
			nonce = values[0]
		}
	}

	if uid > 0 && nonce != "" {
		key := fmt.Sprintf("%d:%s", uid, nonce)
		expireAt := time.Now().Add(30 * time.Second).Unix()
		if _, loaded := replayNonceCache.LoadOrStore(key, expireAt); loaded {
			return nil, fmt.Errorf("replay attack detected")
		}
	}

	return handler(ctx, req)
}
