package router

import (
	"context"
	"log"
	mysqlmodel "spider-server/common/mysql/model"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	"time"
)

type RoomSyncApi struct {
	pb.UnimplementedRoomSyncApiServer
}

func (s *RoomSyncApi) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	user := session.GetUser(ctx)
	err := user.Check()
	if err == nil {
		uid, err := user.UID()
		if err != nil {
			log.Println(err)
		}

		log.Println("玩家 uid", uid)
	} else {
		log.Println("没有 uid", err)
	}

	log.Printf("receive room sync request: %+v", req)
	mysqlmodel.ExampleCreateUser()
	return &pb.SyncResponse{
		Time: uint64(time.Now().UnixMilli()),
	}, nil
}
