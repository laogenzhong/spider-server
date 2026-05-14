package router

import (
	"context"
	"log"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	"time"
)

type RoomSyncApi struct {
	pb.UnimplementedRoomSyncApiServer
}

func (s *RoomSyncApi) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	uid := session.GetUser(ctx).UID()
	log.Println("玩家 uid", uid)
	return &pb.SyncResponse{
		Time: uint64(time.Now().UnixMilli()),
	}, nil
}
