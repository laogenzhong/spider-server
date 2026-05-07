package main

import (
	"context"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "spider-server/gen/spider/api"
)

const grpcAddr = ":50051"

// syncServer 实现 proto 中定义的 RoomSyncApi 服务。
type syncServer struct {
	pb.UnimplementedRoomSyncApiServer
}

// Sync 实现 sync.proto 中的 rpc sync(SyncRequest) returns (SyncResponse)。
func (s *syncServer) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	log.Printf("receive sync request: %+v", req)
	return &pb.SyncResponse{
		Time: uint64(time.Now().UnixMilli()),
	}, nil
}

func main2() {
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRoomSyncApiServer(grpcServer, &syncServer{})

	log.Printf("grpc server listening on %s", grpcAddr)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve grpc server: %v", err)
	}
}
