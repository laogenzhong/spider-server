package game

import (
	"google.golang.org/grpc"
	"log"
	"spider-server/game/service"
	pb "spider-server/gen/spider/api"
)

func (s *GRPCServer) Init() {
	if err := s.Register(func(server *grpc.Server) {
		pb.RegisterRoomSyncApiServer(server, &service.RoomSyncApi{})
	}); err != nil {
		log.Fatalf("register room sync grpc service failed: %v", err)
	}
}
