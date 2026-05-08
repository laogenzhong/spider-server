package router

import (
	"context"
	"log"
	mysqlmodel "spider-server/common/mysql/model"
	pb "spider-server/gen/spider/api"
	"time"
)

type RoomSyncApi struct {
	pb.UnimplementedRoomSyncApiServer
}

func (s *RoomSyncApi) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	log.Printf("receive room sync request: %+v", req)
	mysqlmodel.ExampleCreateUser()
	return &pb.SyncResponse{
		Time: uint64(time.Now().UnixMilli()),
	}, nil
}
