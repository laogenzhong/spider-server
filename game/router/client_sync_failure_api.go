package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type ClientSyncFailureApi struct {
	pb.UnimplementedClientSyncFailureServiceServer
}

func (a *ClientSyncFailureApi) ArchiveClientSyncFailure(
	ctx context.Context,
	req *pb.ArchiveClientSyncFailureRequest,
) (*pb.ArchiveClientSyncFailureResponse, error) {
	// UID 必须取服务端已验证的 Session，不能信任客户端上报值。
	uid := session.GetUser(ctx).UID()
	record := &mysqlmodel.ClientSyncFailure{
		UID:                 uid,
		ClientTaskID:        req.GetClientTaskId(),
		QueueName:           req.GetQueueName(),
		OriginalRPCPath:     req.GetOriginalRpcPath(),
		OriginalRequestBody: req.GetOriginalRequestBody(),
		RequestDataJSON:     readableClientRequestJSON(req),
		BusinessCode:        req.GetBusinessCode(),
		BusinessMessage:     req.GetBusinessMessage(),
		AttemptCount:        req.GetAttemptCount(),
		ClientCreatedAt:     req.GetClientCreatedAt(),
		LastFailedAt:        req.GetLastFailedAt(),
		AppVersion:          req.GetAppVersion(),
		Status:              "pending",
	}

	err := mysqlmodel.ArchiveClientSyncFailure(record)
	if errors.Is(err, mysqlmodel.ErrClientSyncFailureTaskIDEmpty) {
		return session.Error(ctx, gamecode.ClientSyncFailureTaskIDEmpty, &pb.ArchiveClientSyncFailureResponse{})
	}
	if errors.Is(err, mysqlmodel.ErrClientSyncFailurePathEmpty) {
		return session.Error(ctx, gamecode.ClientSyncFailurePathEmpty, &pb.ArchiveClientSyncFailureResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.ClientSyncFailureArchiveFailed, &pb.ArchiveClientSyncFailureResponse{})
	}

	return &pb.ArchiveClientSyncFailureResponse{Stored: true}, nil
}

// readableClientRequestJSON 优先保存客户端提供的业务 JSON；没有时根据 RPC 描述符
// 解码原始 protobuf。无法识别的旧任务仍以 base64 保存，确保数据不丢失。
func readableClientRequestJSON(req *pb.ArchiveClientSyncFailureRequest) string {
	provided := strings.TrimSpace(req.GetRequestDataJson())
	if provided != "" && json.Valid([]byte(provided)) {
		var compact bytes.Buffer
		if json.Compact(&compact, []byte(provided)) == nil {
			return compact.String()
		}
	}

	path := strings.TrimPrefix(strings.TrimSpace(req.GetOriginalRpcPath()), "/")
	separator := strings.LastIndex(path, "/")
	if separator > 0 && separator < len(path)-1 {
		serviceName := path[:separator]
		methodName := path[separator+1:]
		descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
		if err == nil {
			if service, ok := descriptor.(protoreflect.ServiceDescriptor); ok {
				method := service.Methods().ByName(protoreflect.Name(methodName))
				if method != nil {
					messageType, typeErr := protoregistry.GlobalTypes.FindMessageByName(method.Input().FullName())
					if typeErr == nil {
						message := messageType.New().Interface()
						if proto.Unmarshal(req.GetOriginalRequestBody(), message) != nil {
							return fallbackRequestDataJSON(req.GetOriginalRequestBody())
						}
						if data, marshalErr := (protojson.MarshalOptions{
							UseProtoNames:   true,
							EmitUnpopulated: true,
						}).Marshal(message); marshalErr == nil {
							return string(data)
						}
					}
				}
			}
		}
	}

	return fallbackRequestDataJSON(req.GetOriginalRequestBody())
}

func fallbackRequestDataJSON(body []byte) string {
	fallback, _ := json.Marshal(map[string]string{
		"protobuf_base64": base64.StdEncoding.EncodeToString(body),
	})
	return string(fallback)
}
