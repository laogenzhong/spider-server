package game

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"sort"
	appconfig "spider-server/common/config"
	applogger "spider-server/common/logger"
	"spider-server/game/session"
	"strings"
	"sync"
	"time"
)

var replayNonceCache sync.Map
var replayNonceCleanerMu sync.Mutex
var replayNonceCleanerStop chan struct{}
var signVerificationEnabled = appconfig.Default().Sign.Enabled
var replayNonceTTL = appconfig.Default().Sign.ReplayNonceTTLDuration()
var replayNonceCleanupInterval = appconfig.Default().Sign.ReplayNonceCleanupDuration()
var logMetadataPrefixOnly = appconfig.Default().Sign.LogMetadataPrefixOnly

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
	startReplayNonceCleaner(replayNonceCleanupInterval)
}

func ConfigureSign(enabled bool, nonceTTL time.Duration, cleanupInterval time.Duration, metadataPrefixOnly bool) {
	signVerificationEnabled = enabled
	logMetadataPrefixOnly = metadataPrefixOnly
	if nonceTTL > 0 {
		replayNonceTTL = nonceTTL
	}
	if cleanupInterval > 0 {
		replayNonceCleanupInterval = cleanupInterval
		startReplayNonceCleaner(cleanupInterval)
	}
}

func startReplayNonceCleaner(interval time.Duration) {
	if interval <= 0 {
		interval = appconfig.Default().Sign.ReplayNonceCleanupDuration()
	}

	replayNonceCleanerMu.Lock()
	if replayNonceCleanerStop != nil {
		close(replayNonceCleanerStop)
	}
	stop := make(chan struct{})
	replayNonceCleanerStop = stop
	replayNonceCleanerMu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-stop:
				return
			}
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

// MetadataLogInterceptor 打印接口请求内容并校验签名。
func MetadataLogInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	var uid uint64
	if user := session.GetUser(ctx); user != nil {
		uid = user.UIDOrDefault()
	}

	applogger.Printf(
		"interface#uid#%d#methodname#%s#request#%s",
		uid,
		info.FullMethod,
		formatRequestForLog(req),
	)

	if md, ok := metadata.FromIncomingContext(ctx); ok && signVerificationEnabled {
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

	nonce := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("xx-nonce"); len(values) > 0 {
			nonce = values[0]
		}
	}

	if uid > 0 && nonce != "" {
		key := fmt.Sprintf("%d:%s", uid, nonce)
		expireAt := time.Now().Add(replayNonceTTL).Unix()
		if _, loaded := replayNonceCache.LoadOrStore(key, expireAt); loaded {
			return nil, fmt.Errorf("replay attack detected")
		}
	}

	return handler(ctx, req)
}

func formatRequestForLog(req interface{}) string {
	msg, ok := req.(proto.Message)
	if !ok {
		return compactLogText(fmt.Sprintf("%+v", req))
	}
	return compactLogText(formatProtoMessageForLog(msg.ProtoReflect()))
}

func formatProtoMessageForLog(msg protoreflect.Message) string {
	fields := msg.Descriptor().Fields()
	parts := make([]string, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if !messageHasField(msg, field) {
			continue
		}
		parts = append(parts, string(field.TextName())+"="+formatProtoFieldValue(field, msg.Get(field)))
	}
	return strings.Join(parts, ";")
}

func messageHasField(msg protoreflect.Message, field protoreflect.FieldDescriptor) bool {
	switch {
	case field.IsMap():
		return msg.Get(field).Map().Len() > 0
	case field.IsList():
		return msg.Get(field).List().Len() > 0
	default:
		return msg.Has(field)
	}
}

func formatProtoFieldValue(field protoreflect.FieldDescriptor, value protoreflect.Value) string {
	if field.IsMap() {
		return formatProtoMap(value.Map(), field.MapValue())
	}
	if field.IsList() {
		return formatProtoList(value.List(), field)
	}
	return formatProtoScalar(field, value)
}

func formatProtoList(list protoreflect.List, field protoreflect.FieldDescriptor) string {
	values := make([]string, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		values = append(values, formatProtoScalar(field, list.Get(i)))
	}
	return "[" + strings.Join(values, ",") + "]"
}

func formatProtoMap(protoMap protoreflect.Map, valueField protoreflect.FieldDescriptor) string {
	values := make([]string, 0, protoMap.Len())
	protoMap.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		values = append(values, fmt.Sprintf("%s=%s", key.String(), formatProtoScalar(valueField, value)))
		return true
	})
	sort.Strings(values)
	return "{" + strings.Join(values, ",") + "}"
}

func formatProtoScalar(field protoreflect.FieldDescriptor, value protoreflect.Value) string {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return fmt.Sprintf("%t", value.Bool())
	case protoreflect.EnumKind:
		if enumValue := field.Enum().Values().ByNumber(value.Enum()); enumValue != nil {
			return string(enumValue.Name())
		}
		return fmt.Sprintf("%d", value.Enum())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return fmt.Sprintf("%d", value.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return fmt.Sprintf("%d", value.Uint())
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return fmt.Sprintf("%v", value.Float())
	case protoreflect.StringKind:
		return compactLogText(value.String())
	case protoreflect.BytesKind:
		return fmt.Sprintf("<bytes:%d>", len(value.Bytes()))
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "{" + formatProtoMessageForLog(value.Message()) + "}"
	default:
		return compactLogText(fmt.Sprint(value.Interface()))
	}
}

func compactLogText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", ";")
	text = strings.ReplaceAll(text, "\r", ";")
	text = strings.ReplaceAll(text, "\n", ";")
	return text
}
