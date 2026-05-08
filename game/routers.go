package game

import (
	"google.golang.org/grpc"
	"log"
	"spider-server/game/router"
	pb "spider-server/gen/spider/api"
)

func (s *GRPCServer) Init() {
	if err := s.Register(func(server *grpc.Server) {
		pb.RegisterRoomSyncApiServer(server, &router.RoomSyncApi{})
	}); err != nil {
		log.Fatalf("register room sync grpc router failed: %v", err)
	}
}
