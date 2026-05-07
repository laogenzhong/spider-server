package service

import (
	"context"
	"log"
	pb "spider-server/gen/spider/api"
	"time"
)

type RoomSyncApi struct {
	pb.UnimplementedRoomSyncApiServer
}

func (s *RoomSyncApi) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	log.Printf("receive room sync request: %+v", req)

	return &pb.SyncResponse{
		Time: uint64(time.Now().UnixMilli()),
	}, nil
}
